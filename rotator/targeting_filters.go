package rotator

import (
	"math/rand"

	"sort"

	"fmt"

	"bitbucket.org/tapgerine/traffic_rotator/rotator/data"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/redis_handler"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/request_context"
	log "github.com/Sirupsen/logrus"
)

type weightIntervals struct {
	ID         string
	weight     float64
	prevWeight float64
}

type weightIntervalsV2 struct {
	adTag      *AdTagContext
	weight     float64
	prevWeight float64
}

type sortedByERPR []*AdTagContext

func (a sortedByERPR) Len() int {
	return len(a)
}
func (a sortedByERPR) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a sortedByERPR) Less(i, j int) bool {
	if a[i].ERPR > a[j].ERPR {
		return true
	}
	if a[i].ERPR < a[j].ERPR {
		return false
	}
	return a[i].ERPR > a[j].ERPR
}

type sortedByStudy []*AdTagContext

func (a sortedByStudy) Len() int {
	return len(a)
}
func (a sortedByStudy) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a sortedByStudy) Less(i, j int) bool {
	if a[i].StudyLeft > a[j].StudyLeft {
		return true
	}
	if a[i].StudyLeft < a[j].StudyLeft {
		return false
	}
	return a[i].StudyLeft > a[j].StudyLeft
}

func filterAdTagsByPublisher(r request_context.RequestContext, adTags *map[string]data.AdTagData, adTagKeys *[]string) {
	for index, key := range *adTagKeys {
		if key == "" {
			continue
		}
		adTag := (*adTags)[key]
		if adTag.PublisherTargetingID != r.PublisherTargetingID {
			(*adTagKeys)[index] = ""
		}
	}
}

func filterAdTagsForTargeting(r request_context.RequestContext, adTags *map[string]data.AdTagData, adTagKeys *[]string) {
	for index, key := range *adTagKeys {
		if key == "" {
			continue
		}
		adTag := (*adTags)[key]
		if !adTag.CouldBeUsedForTargeting {
			(*adTagKeys)[index] = ""
		}
	}

}

func filterAdTagsByPrice(r request_context.RequestContext, adTags *map[string]data.AdTagData, adTagKeys *[]string) {
	if r.UseOriginPrice {
		// If we use origin price - skip this check
		return
	}

	if r.PublisherPrice == 0 {
		// we do not pay publisher for traffic
		return
	}

	for index, key := range *adTagKeys {
		if key == "" {
			continue
		}
		adTag := (*adTags)[key]
		if !(adTag.Price > 0 && adTag.Price >= r.PublisherPrice) {
			(*adTagKeys)[index] = ""
		}
	}
}

func filterAdTagsByActivity(r request_context.RequestContext, adTags *map[string]data.AdTagData, adTagKeys *[]string) {
	for index, key := range *adTagKeys {
		if key == "" {
			continue
		}
		adTag := (*adTags)[key]
		if !(adTag.IsActive && adTag.IsAdTagPubActive) {
			(*adTagKeys)[index] = ""
		}
	}
}

func filterAdTagsByGeo(r request_context.RequestContext, adTags *map[string]data.AdTagData, adTagKeys *[]string) {
	var hasMatch bool
	for index, key := range *adTagKeys {
		if key == "" {
			continue
		}
		adTag := (*adTags)[key]
		hasMatch = false
		for _, ISOCode := range adTag.Targeting.Geo {
			if ISOCode == "O1" {
				// It means WW targeting
				hasMatch = true
				break
			}
			if r.User.Geo.Country.ISOCode != "" && ISOCode == r.User.Geo.Country.ISOCode {
				hasMatch = true
				break
			}
		}
		if !hasMatch {
			_, historicGeo := adTag.ERPRByGeoForLastWeek[r.User.Geo.Country.ISOCode]

			if !historicGeo {
				if adTag.TotalStats.Requests >= 10000 {
					(*adTagKeys)[index] = ""
				}
			}

		}
	}
}

func filterAdTagsByDeviceType(r request_context.RequestContext, adTags *map[string]data.AdTagData, adTagKeys *[]string) {
	if r.User.UserAgent.DeviceType == "undefined" {
		for index := range *adTagKeys {
			(*adTagKeys)[index] = ""
		}
		return
	}

	for index, key := range *adTagKeys {
		if key == "" {
			continue
		}
		adTag := (*adTags)[key]
		if adTag.Targeting.DeviceType != r.User.UserAgent.DeviceType {
			(*adTagKeys)[index] = ""
		}
	}
}

func selectRandomAdTag(adTags *map[string]data.AdTagData, filteredKeys []string) (string, data.AdTagData) {
	i := rand.Intn(len(filteredKeys))
	selectedKey := filteredKeys[i]

	return selectedKey, (*adTags)[selectedKey]
}

func selectAdTagByERPR(adTags *map[string]data.AdTagData, filteredKeys []string, userGeoCountry string) (string, data.AdTagData) {
	var totalERPR float64

	adTagsWithoutERPR := make([]string, len(filteredKeys))
	adTagsWithERPR := make([]string, len(filteredKeys))
	var adTagsWithoutERPRCounter, adTagsWithERPRCounter int

	for _, adTagID := range filteredKeys {
		adTag := (*adTags)[adTagID]
		adTagERPR, ok := adTag.ERPRByGeoForLastWeek[userGeoCountry]
		if !ok {
			adTagsWithoutERPR[adTagsWithoutERPRCounter] = adTagID
			adTagsWithoutERPRCounter++
		} else {
			adTagsWithERPR[adTagsWithERPRCounter] = adTagID
			totalERPR += adTagERPR.FillRate * adTagERPR.Margin
			adTagsWithERPRCounter++
		}
	}

	var adTagsWithoutERPRChance int
	if adTagsWithoutERPRCounter > len(filteredKeys)/2 {
		adTagsWithoutERPRChance = 50
	} else {
		adTagsWithoutERPRChance = 30
	}

	chanceToWin := rand.Intn(100)

	if adTagsWithoutERPRCounter > 0 && (adTagsWithoutERPRChance > chanceToWin || adTagsWithoutERPRCounter == len(filteredKeys) || adTagsWithERPRCounter == 0) {
		// Ad tags without erpr won and we trying to select one
		i := rand.Intn(adTagsWithoutERPRCounter)
		adTagID := adTagsWithoutERPR[i]
		return adTagID, (*adTags)[adTagID]
	} else {
		erprIntervals := make([]weightIntervals, adTagsWithERPRCounter)
		var totalIntervalsWeight float64

		var i int
		for _, adTagID := range adTagsWithERPR {
			if adTagID == "" {
				continue
			}
			adTagERPR, _ := (*adTags)[adTagID].ERPRByGeoForLastWeek[userGeoCountry]
			prevWeight := totalIntervalsWeight
			totalIntervalsWeight += totalERPR * (adTagERPR.FillRate * adTagERPR.Margin)
			erprIntervals[i] = weightIntervals{
				ID:         adTagID,
				weight:     totalIntervalsWeight,
				prevWeight: prevWeight,
			}
			i++
		}

		randomWeight := rand.Float64() * totalIntervalsWeight

		selectedAdTagID := ""
		for _, interval := range erprIntervals {
			if interval.weight >= randomWeight && randomWeight >= interval.prevWeight {
				selectedAdTagID = interval.ID
				break
			}
		}

		return selectedAdTagID, (*adTags)[selectedAdTagID]
	}
}

func selectManyAdTagByERPR(adTags *map[string]data.AdTagData, filteredKeys []string, userGeoCountry string) map[string]data.AdTagData {
	result := make(map[string]data.AdTagData, 10)

	var i int
	for _, adTagID := range filteredKeys {
		if i == 10 {
			return result
		}
		adTag := (*adTags)[adTagID]
		result[adTagID] = adTag
	}

	return result
}

func selectManyAdTagsByERPRV2(adTags []*AdTagContext, targetingID string, numberOfTags int) []*AdTagContext {
	adTagsForStudy := make([]*AdTagContext, len(adTags))
	adTagsWithERPR := make([]*AdTagContext, len(adTags))
	var adTagsForStudyCounter, adTagsWithERPRCounter int

	var totalERPR float64
	var largestSliceLen int
	for _, adTag := range adTags {
		if adTag.Data.ERPRByTargetingID[targetingID].ERPR > 0 {
			adTagsWithERPR[adTagsWithERPRCounter] = adTag
			adTagsWithERPRCounter++

			if adTagsWithERPRCounter > largestSliceLen {
				largestSliceLen = adTagsWithERPRCounter
			}

			totalERPR += adTag.Data.ERPRByTargetingID[targetingID].ERPR
		} else if adTag.Data.ERPRByTargetingID[targetingID].Requests <= 1000 {
			adTag.StudyLeft = 1000 - adTag.Data.ERPRByTargetingID[targetingID].Requests
			adTagsForStudy[adTagsForStudyCounter] = adTag
			adTagsForStudyCounter++
			if adTagsForStudyCounter > largestSliceLen {
				largestSliceLen = adTagsForStudyCounter
			}
		}
	}

	adTagsWithERPR = adTagsWithERPR[:adTagsWithERPRCounter]
	adTagsForStudy = adTagsForStudy[:adTagsForStudyCounter]

	if adTagsForStudyCounter == 0 {
		sort.Sort(sortedByStudy(adTagsForStudy))
		return adTagsForStudy
	} else if adTagsWithERPRCounter == 0 {
		sort.Sort(sortedByERPR(adTagsWithERPR))
		return adTagsWithERPR
	}

	sort.Sort(sortedByStudy(adTagsForStudy))
	sort.Sort(sortedByERPR(adTagsWithERPR))

	var sortedAdTags []*AdTagContext
	sortedAdTags = make([]*AdTagContext, numberOfTags)
	var sortedAdTagsLen int

	for i := 0; i < largestSliceLen; i++ {
		if i < adTagsWithERPRCounter {
			sortedAdTags[sortedAdTagsLen] = adTagsWithERPR[i]
			sortedAdTagsLen++
		}
		if i < adTagsForStudyCounter {
			sortedAdTags[sortedAdTagsLen] = adTagsForStudy[i]
			sortedAdTagsLen++
		}
		if sortedAdTagsLen >= numberOfTags {
			break
		}
	}
	return sortedAdTags[:sortedAdTagsLen]

}

func selectManyAdTagsByERPR(adTags []*AdTagContext, targetingID string, numberOfTags int) []*AdTagContext {
	var totalERPR float64
	adTagsWithoutERPR := make([]*AdTagContext, len(adTags))
	adTagsWithERPR := make([]*AdTagContext, len(adTags))
	var adTagsWithoutERPRCounter, adTagsWithERPRCounter int
	var largestSliceLen int

	for _, adTag := range adTags {
		adTag.ERPR = adTag.Data.ERPRByTargetingID[targetingID].ERPR

		if adTag.Data.ERPRByTargetingID[targetingID].ERPR > 0 {
			adTagsWithERPR[adTagsWithERPRCounter] = adTag

			adTagsWithERPRCounter++
			if adTagsWithERPRCounter > largestSliceLen {
				largestSliceLen = adTagsWithERPRCounter
			}

			totalERPR += adTag.Data.ERPRByTargetingID[targetingID].ERPR
		} else {
			adTagsWithoutERPR[adTagsWithoutERPRCounter] = adTag
			adTagsWithoutERPRCounter++
			if adTagsWithoutERPRCounter > largestSliceLen {
				largestSliceLen = adTagsWithoutERPRCounter
			}
		}
	}

	adTagsWithERPR = adTagsWithERPR[:adTagsWithERPRCounter]
	adTagsWithoutERPR = adTagsWithoutERPR[:adTagsWithoutERPRCounter]

	if adTagsWithERPRCounter == 0 {
		sort.Sort(sortedByStudy(adTagsWithoutERPR))
		return adTagsWithoutERPR
	} else if adTagsWithoutERPRCounter == 0 {
		sort.Sort(sortedByERPR(adTagsWithERPR))
		return adTagsWithERPR
	}

	sort.Sort(sortedByERPR(adTagsWithERPR))
	sort.Sort(sortedByStudy(adTagsWithoutERPR))

	var sortedAdTags []*AdTagContext
	sortedAdTags = make([]*AdTagContext, numberOfTags)
	var sortedAdTagsLen int

	for i := 0; i < largestSliceLen; i++ {
		if i < adTagsWithERPRCounter {
			sortedAdTags[sortedAdTagsLen] = adTagsWithERPR[i]
			sortedAdTagsLen++
		}
		if i < adTagsWithoutERPRCounter {
			sortedAdTags[sortedAdTagsLen] = adTagsWithoutERPR[i]
			sortedAdTagsLen++
		}
		if sortedAdTagsLen >= numberOfTags {
			break
		}
	}
	return sortedAdTags[:sortedAdTagsLen]

}

func selectAdTagByDomainFillRate(adTags []*AdTagContext, targetingID string, domain string, studyRequests int64) *AdTagContext {
	adTagsForStudy := make([]*AdTagContext, len(adTags))
	adTagsWithFillRate := make([]*AdTagContext, len(adTags))
	var adTagsForStudyCounter, adTagsWithFillRateCounter int

	var totalFillRate float64
	for _, adTag := range adTags {
		if adTag.Data.ERPRByTargetingID[targetingID].FillRate > .0 {
			// We got global fill rate and fill rate per domain, if domain fill rate is higher, we use it instead fill rate

			domainFillRate, exists := adTag.Data.FillRateByTargetingIDAndDomain[targetingID][domain]
			if exists {
				adTag.FillRate = domainFillRate.FillRate
			} else {
				adTag.FillRate = adTag.Data.ERPRByTargetingID[targetingID].FillRate
			}

			adTagsWithFillRate[adTagsWithFillRateCounter] = adTag
			adTagsWithFillRateCounter++
			totalFillRate += adTag.FillRate

		} else if adTag.Data.ERPRByTargetingID[targetingID].Requests <= studyRequests {
			adTagsForStudy[adTagsForStudyCounter] = adTag
			adTagsForStudyCounter++
		}
	}

	adTagsWithFillRate = adTagsWithFillRate[:adTagsWithFillRateCounter]
	adTagsForStudy = adTagsForStudy[:adTagsForStudyCounter]

	if adTagsForStudyCounter > 0 {
		i := rand.Intn(adTagsForStudyCounter)
		adTag := adTagsForStudy[i]
		return adTag
	} else if adTagsWithFillRateCounter > 0 {
		fillRateIntervals := make([]weightIntervalsV2, adTagsWithFillRateCounter)
		var totalIntervalsWeight float64

		var i int
		for _, adTag := range adTagsWithFillRate {
			prevWeight := totalIntervalsWeight
			totalIntervalsWeight += adTag.FillRate
			fillRateIntervals[i] = weightIntervalsV2{
				adTag:      adTag,
				weight:     totalIntervalsWeight,
				prevWeight: prevWeight,
			}
			i++
		}

		randomWeight := rand.Float64() * totalIntervalsWeight

		for _, interval := range fillRateIntervals {
			if interval.weight >= randomWeight && randomWeight >= interval.prevWeight {
				return interval.adTag
			}
		}
	}

	return nil
}

func selectAdTagByFillRate(adTags []*AdTagContext, targetingID string, studyRequests int64) *AdTagContext {
	adTagsForStudy := make([]*AdTagContext, len(adTags))
	adTagsWithFillRate := make([]*AdTagContext, len(adTags))
	var adTagsForStudyCounter, adTagsWithFillRateCounter int

	var totalFillRate float64
	for _, adTag := range adTags {
		if adTag.Data.ERPRByTargetingID[targetingID].FillRate > .0 {
			adTag.FillRate = adTag.Data.ERPRByTargetingID[targetingID].FillRate
			adTagsWithFillRate[adTagsWithFillRateCounter] = adTag
			adTagsWithFillRateCounter++
			totalFillRate += adTag.FillRate

		} else if adTag.Data.ERPRByTargetingID[targetingID].Requests <= studyRequests {
			adTagsForStudy[adTagsForStudyCounter] = adTag
			adTagsForStudyCounter++
		}
	}

	adTagsWithFillRate = adTagsWithFillRate[:adTagsWithFillRateCounter]
	adTagsForStudy = adTagsForStudy[:adTagsForStudyCounter]

	if adTagsForStudyCounter > 0 {
		i := rand.Intn(adTagsForStudyCounter)
		adTag := adTagsForStudy[i]
		return adTag
	} else if adTagsWithFillRateCounter > 0 {
		fillRateIntervals := make([]weightIntervalsV2, adTagsWithFillRateCounter)
		var totalIntervalsWeight float64

		var i int
		for _, adTag := range adTagsWithFillRate {
			prevWeight := totalIntervalsWeight
			totalIntervalsWeight += adTag.FillRate
			fillRateIntervals[i] = weightIntervalsV2{
				adTag:      adTag,
				weight:     totalIntervalsWeight,
				prevWeight: prevWeight,
			}
			i++
		}

		randomWeight := rand.Float64() * totalIntervalsWeight

		for _, interval := range fillRateIntervals {
			if interval.weight >= randomWeight && randomWeight >= interval.prevWeight {
				return interval.adTag
			}
		}
	}

	return nil
}

func selectAdTagByERPRV3(adTags []*AdTagContext, targetingID string, studyRequests int64) *AdTagContext {
	adTagsForStudy := make([]*AdTagContext, len(adTags))
	adTagsWithERPR := make([]*AdTagContext, len(adTags))
	var adTagsForStudyCounter, adTagsWithERPRCounter int

	var totalERPR float64
	for _, adTag := range adTags {
		if adTag.Data.ERPRByTargetingID[targetingID].ERPR > 0 {
			adTagsWithERPR[adTagsWithERPRCounter] = adTag
			adTagsWithERPRCounter++
			totalERPR += adTag.Data.ERPRByTargetingID[targetingID].ERPR
		} else if adTag.Data.ERPRByTargetingID[targetingID].Requests <= studyRequests {
			adTagsForStudy[adTagsForStudyCounter] = adTag
			adTagsForStudyCounter++
		}
	}

	adTagsWithERPR = adTagsWithERPR[:adTagsWithERPRCounter]
	adTagsForStudy = adTagsForStudy[:adTagsForStudyCounter]

	if adTagsForStudyCounter > 0 {
		i := rand.Intn(adTagsForStudyCounter)
		adTag := adTagsForStudy[i]
		return adTag
	} else if adTagsWithERPRCounter > 0 {
		erprIntervals := make([]weightIntervalsV2, adTagsWithERPRCounter)
		var totalIntervalsWeight float64

		var i int
		for _, adTag := range adTagsWithERPR {
			prevWeight := totalIntervalsWeight
			totalIntervalsWeight += adTag.Data.ERPRByTargetingID[targetingID].ERPR
			erprIntervals[i] = weightIntervalsV2{
				adTag:      adTag,
				weight:     totalIntervalsWeight,
				prevWeight: prevWeight,
			}
			i++
		}

		randomWeight := rand.Float64() * totalIntervalsWeight

		for _, interval := range erprIntervals {
			if interval.weight >= randomWeight && randomWeight >= interval.prevWeight {
				return interval.adTag
			}
		}
	}

	return nil
}

func selectAdTagByERPRV2(adTags []*AdTagContext, targetingID string) *AdTagContext {
	adTagsWithoutERPR := make([]*AdTagContext, len(adTags))
	adTagsWithERPR := make([]*AdTagContext, len(adTags))
	var adTagsWithoutERPRCounter, adTagsWithERPRCounter int

	var totalERPR float64
	for _, adTag := range adTags {
		if adTag.Data.ERPRByTargetingID[targetingID].ERPR > 0 {
			adTagsWithERPR[adTagsWithERPRCounter] = adTag
			adTagsWithERPRCounter++
			totalERPR += adTag.Data.ERPRByTargetingID[targetingID].ERPR
		} else {
			adTagsWithoutERPR[adTagsWithoutERPRCounter] = adTag
			adTagsWithoutERPRCounter++
		}
	}

	adTagsWithERPR = adTagsWithERPR[:adTagsWithERPRCounter]
	adTagsWithoutERPR = adTagsWithoutERPR[:adTagsWithoutERPRCounter]

	var adTagsWithoutERPRChance int
	if adTagsWithoutERPRCounter > len(adTags)/2 {
		adTagsWithoutERPRChance = 50
	} else {
		adTagsWithoutERPRChance = 30
	}

	chanceToWin := rand.Intn(100)

	if adTagsWithoutERPRCounter > 0 && (adTagsWithoutERPRChance > chanceToWin || adTagsWithoutERPRCounter == len(adTags) || adTagsWithERPRCounter == 0) {
		// Ad tags without erpr won and we trying to select one
		i := rand.Intn(adTagsWithoutERPRCounter)
		adTag := adTagsWithoutERPR[i]
		return adTag
	} else {
		erprIntervals := make([]weightIntervalsV2, adTagsWithERPRCounter)
		var totalIntervalsWeight float64

		var i int
		for _, adTag := range adTagsWithERPR {
			prevWeight := totalIntervalsWeight
			totalIntervalsWeight += adTag.Data.ERPRByTargetingID[targetingID].ERPR
			erprIntervals[i] = weightIntervalsV2{
				adTag:      adTag,
				weight:     totalIntervalsWeight,
				prevWeight: prevWeight,
			}
			i++
		}

		randomWeight := rand.Float64() * totalIntervalsWeight

		for _, interval := range erprIntervals {
			if interval.weight >= randomWeight && randomWeight >= interval.prevWeight {
				return interval.adTag
			}
		}
	}
	return nil
}

func filterAdTagsByPriceV2(r request_context.RequestContext, adTags []*AdTagContext) {
	if r.UseOriginPrice {
		// If we use origin price - skip this check
		return
	}

	if r.PublisherPrice == 0 {
		// we do not pay publisher for traffic
		return
	}

	for _, adTag := range adTags {
		if !(adTag.Data.Price > 0 && adTag.Data.Price >= r.PublisherPrice) {
			adTag.AllChecksPassed = false
		}
	}
}

func filterAdTagsByRequiredParameters(r request_context.RequestContext, adTags []*AdTagContext) {
	for _, adTag := range adTags {
		if adTag.AllChecksPassed == false {
			continue
		}

		parametersMapping, err := data.ServingData.GetParametersMapByID(adTag.Data.AdvertiserPlatformTypeID)
		if err != nil {
			log.WithField("ad_tag_id", adTag.ID).WithError(err).Warn()
			continue
		}

		for _, parameter := range parametersMapping {
			if parameter[r.RequestPlatform].IsRequired {
				valueFromRequest := r.Request.URL.Query().Get(parameter[r.RequestPlatform].OriginalShortcut)
				if valueFromRequest == "" || valueFromRequest == parameter[r.RequestPlatform].OriginalMacros {
					adTag.AllChecksPassed = false
					break
				}
			}
		}
	}
}

func filterAdTagsByDomainLists(r request_context.RequestContext, adTags []*AdTagContext) {
	for _, adTag := range adTags {
		if adTag.AllChecksPassed == false {
			continue
		}
		if adTag.Data.DomainsListID > 0 {
			if r.Domain == "" {
				adTag.AllChecksPassed = false
				continue
			}

			domainsListItem, err := redis_handler.RedisConnection.HGet(fmt.Sprintf("domains:%d", adTag.Data.DomainsListID), r.Domain).Result()

			if adTag.Data.DomainsListType == "white" && !(err == nil && domainsListItem == "white") {
				// White list activated. Domain is not in the list
				adTag.AllChecksPassed = false
				continue
			} else if adTag.Data.DomainsListType == "black" && err == nil && domainsListItem == "black" {
				// Black list activated. Domain is in the list
				adTag.AllChecksPassed = false
				continue
			}
		}
	}
}

func filterAdTagsByDeviceTypeV2(r request_context.RequestContext, adTags []*AdTagContext) {
	if r.User.UserAgent.DeviceType == "undefined" {
		return
	}

	for _, adTag := range adTags {
		if adTag.Data.Targeting.DeviceType != r.User.UserAgent.DeviceType {
			adTag.AllChecksPassed = false
		}
	}
}

func filterAdTagsByGeoV2(r request_context.RequestContext, adTags []*AdTagContext) {
	for _, adTag := range adTags {
		if adTag.AllChecksPassed == false {
			continue
		}
		adTag.GeoConfig.IsSetUpGeo = false
		for _, ISOCode := range adTag.Data.Targeting.Geo {
			if ISOCode == "O1" {
				// It means WW targeting
				adTag.GeoConfig.IsWorldWide = true
				break
			}
			if r.User.Geo.Country.ISOCode != "" && ISOCode == r.User.Geo.Country.ISOCode {
				adTag.GeoConfig.IsSetUpGeo = true
				break
			}
		}
		if !adTag.GeoConfig.IsWorldWide && !adTag.GeoConfig.IsSetUpGeo {
			_, historicGeo := adTag.Data.ERPRByGeoForLastWeek[r.User.Geo.Country.ISOCode]

			if !historicGeo {
				if adTag.Data.ERPRByTargetingID[r.PublisherTargetingID].Requests <= 10000 {
					adTag.IsPeriodOfStudy = true
					adTag.StudyLeft = 10000 - adTag.Data.ERPRByTargetingID[r.PublisherTargetingID].Requests
				} else {
					adTag.IsPeriodOfStudyPassed = true
					adTag.AllChecksPassed = false
				}
			} else {
				adTag.GeoConfig.IsHistoricalGeo = true
			}
		}
	}
}

func filterAdTagsByGeoV3(r request_context.RequestContext, adTags []*AdTagContext) {
	for _, adTag := range adTags {
		if adTag.AllChecksPassed == false {
			continue
		}
		adTag.GeoConfig.IsSetUpGeo = false
		for _, ISOCode := range adTag.Data.Targeting.Geo {
			if ISOCode == "O1" {
				// It means WW targeting
				adTag.GeoConfig.IsWorldWide = true
				break
			}
			if r.User.Geo.Country.ISOCode != "" && ISOCode == r.User.Geo.Country.ISOCode {
				adTag.GeoConfig.IsSetUpGeo = true
				break
			}
		}

		if !adTag.GeoConfig.IsWorldWide && !adTag.GeoConfig.IsSetUpGeo {
			adTag.GeoCheckFailed = true
			adTag.AllChecksPassed = false
		}

		//if !adTag.GeoConfig.IsWorldWide && !adTag.GeoConfig.IsSetUpGeo {
		//	_, historicGeo := adTag.Data.ERPRByGeoForLastWeek[r.User.Geo.Country.ISOCode]
		//
		//	if !historicGeo {
		//		if adTag.Data.ERPRByTargetingID[r.PublisherTargetingID].Requests <= 10000 {
		//			adTag.IsPeriodOfStudy = true
		//			adTag.StudyLeft = 10000 - adTag.Data.ERPRByTargetingID[r.PublisherTargetingID].Requests
		//		} else {
		//			adTag.IsPeriodOfStudyPassed = true
		//			adTag.AllChecksPassed = false
		//		}
		//	} else {
		//		adTag.GeoConfig.IsHistoricalGeo = true
		//	}
		//}
	}
}
