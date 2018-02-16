package rotator

import (
	"net/http"
	"net/url"
	"time"

	"encoding/json"

	"bitbucket.org/tapgerine/traffic_rotator/rotator/data"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/request_context"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/vast"
	log "github.com/Sirupsen/logrus"
	"github.com/satori/go.uuid"
)

var EncryptionKey []byte

type singlePageResponse struct {
	IP string `json:"ip"`
	UA string `json:"ua"`
}

func SinglePageUserData(w http.ResponseWriter, r *http.Request) {
	requestContext := &request_context.RequestContext{}
	requestContext.ParseIP("", r)

	response, _ := json.Marshal(singlePageResponse{requestContext.User.IP.String(), r.UserAgent()})
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func AdRotationHandler(w http.ResponseWriter, r *http.Request) {
	timestamp := time.Now().UTC()

	// Logging request into redis
	//hr, min, _ := timestamp.Clock()
	//redis_handler.RedisConnection.HIncrBy(
	//	fmt.Sprintf("requests:%s", timestamp.Format("2006-01-02")),
	//	strconv.Itoa(hr*60+min),
	//	1,
	//)

	adTagPubID := r.URL.Query().Get("adtagpubid")
	if adTagPubID == "" {
		w.WriteHeader(204)
		log.WithField("url", r.URL.String()).Warn("No adtagpubid in request")
		return
	}
	defer r.Body.Close()

	adTag, err := data.ServingData.GetAdTagByID(adTagPubID)

	if err != nil {
		w.WriteHeader(204)
		return
	}

	if (!adTag.IsActive || !adTag.IsAdTagPubActive) && !adTag.IsTest {
		w.WriteHeader(204)
		w.Header().Set("X-Not-Active", "true")
		return
	}

	requestContext := &request_context.RequestContext{
		Type:      "direct",
		RequestID: uuid.NewV4(),
		User:      request_context.UserContext{},
	}

	// Trying to resolve user data
	ipParameterMapping, err := data.ServingData.GetOurPlatformParametersMapByNameAndID("ip", adTag.AdvertiserPlatformTypeID, "desktop")
	if err != nil {
		//log.WithField("url", r.URL.String()).WithError(err).Warn("Can't find mapping")
	} else {
		requestContext.ParseIP(r.URL.Query().Get(ipParameterMapping.Shortcut), r)
		//requestContext.User.IP = net.ParseIP(r.URL.Query().Get(ipParameterMapping.Shortcut))
		requestContext.ParseGeo()
	}
	uaParameterMapping, err := data.ServingData.GetOurPlatformParametersMapByNameAndID("ua", adTag.AdvertiserPlatformTypeID, "desktop")
	if err != nil {
		//log.WithField("url", r.URL.String()).WithError(err).Warn("Can't find mapping")
	} else {
		requestContext.User.UserAgentString = r.URL.Query().Get(uaParameterMapping.Shortcut)
		requestContext.ParseUserAgent()
	}
	urlParameterMapping, err := data.ServingData.GetOurPlatformParametersMapByNameAndID("url", adTag.AdvertiserPlatformTypeID, "desktop")
	if err != nil {
		//log.WithField("url", r.URL.String()).WithError(err).Warn("Can't find mapping")
	} else {
		requestContext.ParseDomain(r.URL.Query().Get(urlParameterMapping.Shortcut), r)
	}

	// Packing new url
	originalURL, err := url.Parse(adTag.URL)
	newURL := url.URL{}
	mergedQuery := newURL.Query()

	for key := range originalURL.Query() {
		mergedQuery.Add(key, r.URL.Query().Get(key))
	}
	originalURL.RawQuery = mergedQuery.Encode()

	SendRequestMessageToKafka(adTagPubID, requestContext.RequestID, timestamp,
		requestContext.User.Geo.Country.ISOCode, requestContext.User.UserAgent.DeviceType,
		requestContext.Domain,
	)

	if adTag.SupportsVast {
		generatedVast, err := vast.GenerateVASTWrapper(
			adTag.AdTagID,
			originalURL.String(),
			requestContext.RequestID.String(),
			adTagPubID,
			adTag.Price,
			EncryptionKey,
			requestContext.Type,
			requestContext.User.Geo.Country.ISOCode,
			requestContext.User.UserAgent.DeviceType,
			"", requestContext.Domain, "", "", 2,
		)
		if err != nil {
			w.WriteHeader(204)
			// TODO: add error header mb?
			log.WithField("url", r.URL.String()).WithError(err).Warn()
			return
		}

		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(generatedVast))
	} else {
		http.Redirect(w, r, originalURL.String(), 302)
	}

	//timePassed := time.Now().UTC().Sub(timestamp)
	//redis_handler.RedisConnection.HIncrBy(
	//	fmt.Sprintf("requests:%s:time", timestamp.Format("2006-01-02")),
	//	getLogTimePassed(timePassed),
	//	1,
	//)
}
