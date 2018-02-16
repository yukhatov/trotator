package rotator

import (
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"fmt"

	"net/url"

	"bitbucket.org/tapgerine/traffic_rotator/rotator/data"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/redis_handler"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/request_context"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/vast"
	log "github.com/Sirupsen/logrus"
)

var defaultDomainBlackListKey = "domains:12"

type AdTagContext struct {
	ID                    string
	Data                  data.AdTagData
	AllChecksPassed       bool
	GeoConfig             AdTagContextGeo
	IsPeriodOfStudy       bool
	IsPeriodOfStudyPassed bool
	GeoCheckFailed        bool
	StudyLeft             int64
	ERPR                  float64
	FillRate              float64
}

type AdTagContextGeo struct {
	IsWorldWide     bool
	IsSetUpGeo      bool
	IsHistoricalGeo bool
}

func AdRotationTargetingHandlerV2(w http.ResponseWriter, r *http.Request) {
	timestamp := time.Now().UTC()

	// Logging request into redis
	hr, min, _ := timestamp.Clock()
	redis_handler.RedisConnection.HIncrBy(
		fmt.Sprintf("requests:targeting_v2:%s", timestamp.Format("2006-01-02")),
		strconv.Itoa(hr*60+min),
		1,
	)

	requestContext, err := parseRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		log.WithField("url", r.URL.String()).Warn(err)
		return
	}

	publisherLink := &PublisherLink{}
	err = publisherLink.init(requestContext.PublisherTargetingID)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	requestContext.SetRequestPlatform(publisherLink.Data.Platform)
	if requestContext.PriceParsingError == ErrPriceParsing {
		if publisherLink.Data.Price > 0.0 {
			requestContext.PublisherPrice = publisherLink.Data.Price
		} else {
			w.WriteHeader(http.StatusNoContent)
			log.WithField("url", r.URL.String()).Warn(fmt.Sprintf("Price was not set, targeting_id = %s", requestContext.PublisherTargetingID))
			return
		}
	}

	// Checking default domain black list
	domainsListItem, err := redis_handler.RedisConnection.HGet(defaultDomainBlackListKey, requestContext.Domain).Result()
	if err == nil && domainsListItem == "black" {
		// Black list activated. Domain is in the list
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Check for domain black/white list
	isAllowed := publisherLink.IsDomainAllowForThisLink(requestContext.Domain)
	if !isAllowed {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Getting a list of available ad tag ids for this publisher link
	adTagIDs, err := publisherLink.GetAdTagIDs()
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	adTags, err := data.ServingData.GetAdTagsByIDs(adTagIDs)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var adTagContextList []*AdTagContext
	adTagContextList = make([]*AdTagContext, len(adTags))
	var i int
	for id, tag := range adTags {
		adTagContextList[i] = &AdTagContext{ID: id, Data: tag, AllChecksPassed: true}
		i++
	}

	filterAdTagsByPriceV2(requestContext, adTagContextList)

	if requestContext.RequestPlatform == "desktop" {
		filterAdTagsByDeviceTypeV2(requestContext, adTagContextList)
	}

	filterAdTagsByRequiredParameters(requestContext, adTagContextList)
	filterAdTagsByDomainLists(requestContext, adTagContextList)
	// For now geo check should be last one
	filterAdTagsByGeoV3(requestContext, adTagContextList)
	//filterAdTagsByGeoV2(requestContext, adTagContextList)

	var tagsPassedFiltersCount int
	var tagsNotPassedGeoFilterCount int
	for _, adTag := range adTagContextList {
		if adTag.AllChecksPassed {
			tagsPassedFiltersCount++
		} else if adTag.GeoCheckFailed {
			tagsNotPassedGeoFilterCount++
		}
	}

	var adTagContextAfterFilters []*AdTagContext
	var isOnlyTagsWithGeoCheckFailedLeft bool
	var adTagContextAfterFiltersLen int
	if tagsPassedFiltersCount == 0 && tagsNotPassedGeoFilterCount > 0 {
		adTagContextAfterFiltersLen = tagsNotPassedGeoFilterCount
		isOnlyTagsWithGeoCheckFailedLeft = true
	} else {
		adTagContextAfterFiltersLen = tagsPassedFiltersCount
	}
	adTagContextAfterFilters = make([]*AdTagContext, adTagContextAfterFiltersLen)

	var adTagContextAfterFiltersCount int
	for _, adTag := range adTagContextList {
		// Even if all tags failed checks except last one (geo), we still could select some tags for serving
		if tagsPassedFiltersCount == 0 && tagsNotPassedGeoFilterCount > 0 && adTag.GeoCheckFailed {
			adTagContextAfterFilters[adTagContextAfterFiltersCount] = adTag
			adTagContextAfterFiltersCount++
		} else if adTag.AllChecksPassed {
			adTagContextAfterFilters[adTagContextAfterFiltersCount] = adTag
			adTagContextAfterFiltersCount++
		}
	}

	if adTagContextAfterFiltersCount == 0 {
		w.WriteHeader(http.StatusNoContent)
		notifyKafkaAboutEmptyResponse(requestContext, timestamp)
		return
	}

	var response string

	if requestContext.ResponseType == "vast" {
		var selectedAdTag *AdTagContext
		if isOnlyTagsWithGeoCheckFailedLeft {
			i := rand.Intn(adTagContextAfterFiltersCount)
			selectedAdTag = adTagContextAfterFilters[i]
		} else {
			if publisherLink.Data.Optimization == "erpr" {
				selectedAdTag = selectAdTagByERPRV3(adTagContextAfterFilters, requestContext.PublisherTargetingID, publisherLink.Data.StudyRequests)
			} else if publisherLink.Data.Optimization == "fill_rate" {
				selectedAdTag = selectAdTagByFillRate(adTagContextAfterFilters, requestContext.PublisherTargetingID, publisherLink.Data.StudyRequests)
			} else {
				selectedAdTag = selectAdTagByDomainFillRate(adTagContextAfterFilters, requestContext.PublisherTargetingID, requestContext.Domain, publisherLink.Data.StudyRequests)
			}

			//if requestContext.PublisherTargetingID == "OaIsmaWJ" || requestContext.PublisherTargetingID == "rKbNUciT" {
			//	selectedAdTag = selectAdTagByERPRV4(adTagContextAfterFilters, requestContext.PublisherTargetingID, requestContext.Domain)
			//} else {
			//	selectedAdTag = selectAdTagByERPRV3(adTagContextAfterFilters, requestContext.PublisherTargetingID)
			//}
		}
		// TODO: if no tags selected - choose random
		if selectedAdTag == nil {
			w.WriteHeader(http.StatusNoContent)
			notifyKafkaAboutEmptyResponse(requestContext, timestamp)
			return
		}

		response, err = generateVASTResponse(requestContext, selectedAdTag.Data, selectedAdTag.ID)
		if err != nil {
			w.WriteHeader(http.StatusNoContent)
			notifyKafkaAboutEmptyResponse(requestContext, timestamp)
			log.WithField("url", r.URL.String()).WithError(err).Warn()
			return
		}
		SendRequestTargetedMessageToKafka(
			selectedAdTag.ID, requestContext.RequestID, timestamp, requestContext.User.Geo.Country.ISOCode,
			requestContext.DevicePlatformType, selectedAdTag.Data.PublisherID, "targeting",
			requestContext.PublisherTargetingID, requestContext.Domain,
			requestContext.AppName, requestContext.BundleID,
		)

	} else if requestContext.ResponseType == "vpaid" {
		var selectedAdTags []*AdTagContext
		if isOnlyTagsWithGeoCheckFailedLeft {
			// TODO: this about limitation
			selectedAdTags = adTagContextAfterFilters
		} else {
			selectedAdTags = selectManyAdTagsByERPRV2(adTagContextAfterFilters, requestContext.PublisherTargetingID, 10)
		}

		selectedAdTagsMap := make(map[string]data.AdTagData, len(selectedAdTags))
		for _, adTag := range selectedAdTags {
			// TODO: check how adTag.Data (data.AdTagData) is passed to this place, check for memory leaks and races
			selectedAdTagsMap[adTag.ID] = adTag.Data
		}

		response, err = generateVASTVPAIDResponse(requestContext, selectedAdTagsMap)

		if err != nil {
			w.WriteHeader(http.StatusNoContent)
			// TODO: add error header mb?
			log.WithField("url", r.URL.String()).WithError(err).Warn()
			return
		}
		publisherID, err := data.ServingData.GetPublisherIDByTargetingID(requestContext.PublisherTargetingID)
		if err != nil {
			//w.WriteHeader(http.StatusNoContent)
			log.WithField("url", r.URL.String()).Warn(err)
			return
		}
		SendRequestTargetedMessageToKafka(
			"", requestContext.RequestID, timestamp, requestContext.User.Geo.Country.ISOCode,
			requestContext.DevicePlatformType, publisherID, "vpaid",
			requestContext.PublisherTargetingID, requestContext.Domain,
			requestContext.AppName, requestContext.BundleID,
		)
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(response))

	timePassed := time.Now().UTC().Sub(timestamp)

	redis_handler.RedisConnection.HIncrBy(
		fmt.Sprintf("requests:targeting:%s:time", timestamp.Format("2006-01-02")),
		getLogTimePassed(timePassed),
		1,
	)

}

func notifyKafkaAboutEmptyResponse(requestContext request_context.RequestContext, timestamp time.Time) {
	publisherID, err := data.ServingData.GetPublisherIDByTargetingID(requestContext.PublisherTargetingID)
	if err != nil {
		//w.WriteHeader(http.StatusNoContent)
		log.WithField("url", requestContext.Request.URL.String()).Warn(err)
		return
	}
	// TODO: this is hack, we need more elegant solution
	requestType := "targeting"
	if requestContext.ResponseType == "vpaid" {
		requestType = "vpaid"
	}

	SendRequestTargetedMessageToKafka(
		"", requestContext.RequestID, timestamp, requestContext.User.Geo.Country.ISOCode,
		requestContext.DevicePlatformType, publisherID, requestType,
		requestContext.PublisherTargetingID, requestContext.Domain,
		requestContext.AppName, requestContext.BundleID,
	)
	timePassed := time.Now().UTC().Sub(timestamp)
	redis_handler.RedisConnection.HIncrBy(
		fmt.Sprintf("requests:targeting:%s:time", timestamp.Format("2006-01-02")),
		getLogTimePassed(timePassed),
		1,
	)
	return
}

// AdRotationTargetingHandler handle request from publishers and supports targeting
func AdRotationTargetingHandler(w http.ResponseWriter, r *http.Request) {
	timestamp := time.Now().UTC()

	// Logging request into redis
	hr, min, _ := timestamp.Clock()
	redis_handler.RedisConnection.HIncrBy(
		fmt.Sprintf("requests:targeting:%s", timestamp.Format("2006-01-02")),
		strconv.Itoa(hr*60+min),
		1,
	)

	requestContext, err := parseRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		log.WithField("url", r.URL.String()).Warn(err)
		return
	}

	adTags, err := data.ServingData.GetAllAdTags()
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		log.WithField("url", r.URL.String()).Warn(err)
		return
	}

	var adTagsKeys []string
	adTagsKeys = make([]string, len(adTags))
	var i int
	for key := range adTags {
		adTagsKeys[i] = key
		i++
	}

	filterAdTagsByPublisher(requestContext, &adTags, &adTagsKeys)
	filterAdTagsForTargeting(requestContext, &adTags, &adTagsKeys)
	filterAdTagsByActivity(requestContext, &adTags, &adTagsKeys)
	filterAdTagsByPrice(requestContext, &adTags, &adTagsKeys)
	filterAdTagsByGeo(requestContext, &adTags, &adTagsKeys)
	filterAdTagsByDeviceType(requestContext, &adTags, &adTagsKeys)

	var adTagsKeysAfterFilter []string
	for _, key := range adTagsKeys {
		if key != "" {
			adTagsKeysAfterFilter = append(adTagsKeysAfterFilter, key)
		}
	}

	if len(adTagsKeysAfterFilter) == 0 {
		w.WriteHeader(http.StatusNoContent)
		publisherID, err := data.ServingData.GetPublisherIDByTargetingID(requestContext.PublisherTargetingID)
		if err != nil {
			//w.WriteHeader(http.StatusNoContent)
			log.WithField("url", r.URL.String()).Warn(err)
			return
		}
		// TODO: this is hack, we need more elegant solution
		requestType := "targeting"
		if requestContext.ResponseType == "vpaid" {
			requestType = "vpaid"
		}

		SendRequestTargetedMessageToKafka(
			"", requestContext.RequestID, timestamp, requestContext.User.Geo.Country.ISOCode,
			requestContext.User.UserAgent.DeviceType, publisherID, requestType,
			requestContext.PublisherTargetingID, requestContext.Domain,
			requestContext.AppName, requestContext.BundleID,
		)
		timePassed := time.Now().UTC().Sub(timestamp)
		redis_handler.RedisConnection.HIncrBy(
			fmt.Sprintf("requests:targeting:%s:time", timestamp.Format("2006-01-02")),
			getLogTimePassed(timePassed),
			1,
		)
		return
	}

	var response string

	if requestContext.ResponseType == "vast" {
		adTagPubID, adTag := selectAdTagByERPR(&adTags, adTagsKeysAfterFilter, requestContext.User.Geo.Country.ISOCode)
		response, err = generateVASTResponse(requestContext, adTag, adTagPubID)
		if err != nil {
			w.WriteHeader(http.StatusNoContent)
			// TODO: add error header mb?
			log.WithField("url", r.URL.String()).WithError(err).Warn()
			return
		}
		SendRequestTargetedMessageToKafka(
			adTagPubID, requestContext.RequestID, timestamp, requestContext.User.Geo.Country.ISOCode,
			requestContext.User.UserAgent.DeviceType, adTag.PublisherID, "targeting",
			requestContext.PublisherTargetingID, requestContext.Domain,
			requestContext.AppName, requestContext.BundleID,
		)

	} else if requestContext.ResponseType == "vpaid" {
		adTagsMap := selectManyAdTagByERPR(&adTags, adTagsKeysAfterFilter, requestContext.User.Geo.Country.ISOCode)
		response, err = generateVASTVPAIDResponse(requestContext, adTagsMap)

		if err != nil {
			w.WriteHeader(http.StatusNoContent)
			// TODO: add error header mb?
			log.WithField("url", r.URL.String()).WithError(err).Warn()
			return
		}
		publisherID, err := data.ServingData.GetPublisherIDByTargetingID(requestContext.PublisherTargetingID)
		if err != nil {
			//w.WriteHeader(http.StatusNoContent)
			log.WithField("url", r.URL.String()).Warn(err)
			return
		}
		SendRequestTargetedMessageToKafka(
			"", requestContext.RequestID, timestamp, requestContext.User.Geo.Country.ISOCode,
			requestContext.User.UserAgent.DeviceType, publisherID, "vpaid",
			requestContext.PublisherTargetingID, requestContext.Domain,
			requestContext.AppName, requestContext.BundleID,
		)
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(response))

	timePassed := time.Now().UTC().Sub(timestamp)

	redis_handler.RedisConnection.HIncrBy(
		fmt.Sprintf("requests:targeting:%s:time", timestamp.Format("2006-01-02")),
		getLogTimePassed(timePassed),
		1,
	)
}

func generateVASTVPAIDResponse(requestContext request_context.RequestContext, adTagsMap map[string]data.AdTagData) (string, error) {
	mappedURLs := make(map[string]string, len(adTagsMap))
	for adTagID, adTag := range adTagsMap {
		originalURL, err := mapUrl(adTag.URL, *requestContext.Request.URL, adTag.AdvertiserPlatformTypeID, requestContext.RequestPlatform, requestContext)
		if err != nil {
			continue
		}
		mappedURLs[adTagID] = originalURL.String()
	}

	return vast.GenerateVASTVPAID(adTagsMap, mappedURLs, requestContext, EncryptionKey)
}

func generateVASTResponse(requestContext request_context.RequestContext, adTag data.AdTagData, adTagPubID string) (string, error) {
	originalURL, err := mapUrl(adTag.URL, *requestContext.Request.URL, adTag.AdvertiserPlatformTypeID, requestContext.RequestPlatform, requestContext)

	if err != nil {
		return "", err
	}

	var price float64
	if requestContext.UseOriginPrice {
		price = adTag.Price
	} else {
		price = requestContext.PublisherPrice
	}

	return vast.GenerateVASTWrapper(
		adTag.AdTagID,
		originalURL.String(),
		requestContext.RequestID.String(),
		adTagPubID,
		price,
		EncryptionKey,
		requestContext.Type,
		requestContext.User.Geo.Country.ISOCode,
		requestContext.User.UserAgent.DeviceType,
		requestContext.PublisherTargetingID,
		requestContext.Domain,
		requestContext.AppName,
		requestContext.BundleID,
		requestContext.VastVersion,
	)
	//if err != nil {
	//	w.WriteHeader(http.StatusNoContent)
	//	// TODO: add error header mb?
	//	log.WithField("url", r.URL.String()).WithError(err).Warn()
	//	return
	//}

	//return generatedVast, err
}

func mapUrl(urlToMap string, requestUrl url.URL, mapperID uint64, platform string, requestContext request_context.RequestContext) (url.URL, error) {
	// TODO: if their value is not macros - do not replace
	parametersMapping, err := data.ServingData.GetParametersMapByID(mapperID)
	if err != nil {
		return url.URL{}, err
	}

	originalURL, err := url.Parse(urlToMap)
	newURL := url.URL{}
	mergedQuery := newURL.Query()

	for key, value := range originalURL.Query() {
		mapping, hasMapping := parametersMapping[key]
		if hasMapping {
			mappingForCurrentPlatform := mapping[platform]
			valueFromRequest := requestUrl.Query().Get(mappingForCurrentPlatform.OriginalShortcut)

			if mappingForCurrentPlatform.OriginalShortcut == "ua" {
				mergedQuery.Add(key, requestContext.User.UserAgentString)
			} else if mappingForCurrentPlatform.OriginalShortcut == "ip" {
				if requestContext.User.IP != nil {
					mergedQuery.Add(key, requestContext.User.IP.String())
				}
			} else if mappingForCurrentPlatform.OriginalShortcut == "url" {
				mergedQuery.Add(key, requestContext.Referrer)
			} else if valueFromRequest != "" && valueFromRequest != mappingForCurrentPlatform.OriginalMacros {
				mergedQuery.Add(key, valueFromRequest)
				continue
			}
		} else {
			mergedQuery.Add(key, value[0])
		}
	}

	originalURL.RawQuery = mergedQuery.Encode()

	return *originalURL, nil
}
