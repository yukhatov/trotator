package rotator

import (
	"net/http"
	"time"

	"bitbucket.org/tapgerine/traffic_rotator/rotator/redis_handler"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/vast"
	log "github.com/Sirupsen/logrus"
)

func AdRotationOpenRTBInitHandler(w http.ResponseWriter, r *http.Request) {
	timestamp := time.Now().UTC()

	requestContext, err := parseRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		log.WithField("url", r.URL.String()).Warn(err)
		SendRTBEventMessageToKafka(requestContext, "init_error", timestamp)
		return
	}

	var publisherLink PublisherLink
	if err = publisherLink.init(requestContext.PublisherTargetingID); err != nil {
		w.WriteHeader(http.StatusNoContent)
		SendRTBEventMessageToKafka(requestContext, "init_error", timestamp)
		return
	}
	requestContext.SetRequestPlatform(publisherLink.Data.Platform)
	publisherID, err := publisherLink.GetPublisherID()
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		log.WithField("url", r.URL.String()).WithError(err).Warn()
		SendRTBEventMessageToKafka(requestContext, "init_error", timestamp)
		return
	}
	requestContext.PublisherID = publisherID

	// Checking default domain black list
	domainsListItem, err := redis_handler.RedisConnection.HGet(defaultDomainBlackListKey, requestContext.Domain).Result()
	if err == nil && domainsListItem == "black" {
		// Black list activated. Domain is in the list
		w.WriteHeader(http.StatusNoContent)
		SendRTBEventMessageToKafka(requestContext, "init_error", timestamp)
		return
	}

	// Check for domain black/white list
	isAllowed := publisherLink.IsDomainAllowForThisLink(requestContext.Domain)
	if !isAllowed {
		w.WriteHeader(http.StatusNoContent)
		SendRTBEventMessageToKafka(requestContext, "init_error", timestamp)
		return
	}

	response, err := vast.GenerateVASTVPAIDForOpenRTB(requestContext)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		log.WithField("url", r.URL.String()).WithError(err).Warn()
		SendRTBEventMessageToKafka(requestContext, "init_error", timestamp)
		return
	}

	SendRTBEventMessageToKafka(requestContext, "init", timestamp)

	w.Header().Set("Content-Type", "application/xml")
	//w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write([]byte(response))
	return
}
