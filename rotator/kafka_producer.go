package rotator

import (
	"time"

	"encoding/json"

	"bitbucket.org/tapgerine/traffic_rotator/rotator/message_format"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/request_context"
	"github.com/Shopify/sarama"
	log "github.com/Sirupsen/logrus"
	"github.com/satori/go.uuid"
)

var (
	KafkaProducer sarama.AsyncProducer
)

type KafkaRequestMessageFormat struct {
	MessageType string `json:"message_type"`
	AdTagPubID  string `json:"adpid"`
	RequestID   string `json:"rid"`
	Timestamp   int64  `json:"timestamp"`
	RequestType string `json:"rtype"`
	GeoCountry  string `json:"geo_country"`
	DeviceType  string `json:"device_type"`
	PublisherID uint64 `json:"publisher_id"`
	TargetingID string `json:"targeting_id"`
	Domain      string `json:"domain"`
	AppName     string `json:"app_name"`
	BundleID    string `json:"bundle_id"`
}

//type KafkaRTBEventsMessageFormat struct {
//	ID             string  `json:"id"`
//	PublisherID    uint64  `json:"pid"`
//	TargetingID    string  `json:"tid"`
//	EventName      string  `json:"e"`
//	PublisherPrice float64 `json:"price"`
//	Timestamp      int64   `json:"timestamp"`
//	GeoCountry     string  `json:"geo_country"`
//	DeviceType     string  `json:"device_type"`
//	Domain         string  `json:"domain"`
//	AppName        string  `json:"app_name"`
//	BundleID       string  `json:"bundle_id"`
//}

type KafkaRTBBidRequestsMessageFormat struct {
	ID                 string  `json:"id"`
	PublisherID        uint64  `json:"pid"`
	TargetingID        string  `json:"tid"`
	PublisherPrice     float64 `json:"price"`
	Timestamp          int64   `json:"timestamp"`
	AdvertiserID       uint64  `json:"advertiser_id"`
	BidResponse        int8    `json:"bid_response"`
	BidResponseTime    int64   `json:"bid_response_time"`
	BidResponseTimeout int8    `json:"bid_response_timeout"`
	BidResponseEmpty   int8    `json:"bid_response_empty"`
	BidResponseError   string  `json:"bid_response_error"`
	BidWin             int8    `json:"bid_win"`
	BidFloorPrice      float64 `json:"bid_floor_price"`
	BidPrice           float64 `json:"bid_price"`
	SecondPrice        float64 `json:"second_price"`
	GeoCountry         string  `json:"geo_country"`
	DeviceType         string  `json:"device_type"`
	Domain             string  `json:"domain"`
	AppName            string  `json:"app_name"`
	BundleID           string  `json:"bundle_id"`
}

func SendRequestMessageToKafka(
	adTagPubID string, requestID uuid.UUID, timestamp time.Time,
	geoCountry, deviceType string, domain string,
) {
	msg := KafkaRequestMessageFormat{
		AdTagPubID:  adTagPubID,
		RequestID:   requestID.String(),
		Timestamp:   timestamp.Unix(),
		GeoCountry:  geoCountry,
		DeviceType:  deviceType,
		RequestType: "direct",
		Domain:      domain,
	}

	msgJson, err := json.Marshal(msg)
	if err != nil {
		log.WithError(err).Warn("Can't marshall kafka message")
	}

	message := &sarama.ProducerMessage{
		Topic:     "requests",
		Partition: 0,
		Value:     sarama.StringEncoder(msgJson),
	}
	KafkaProducer.Input() <- message
}

func SendRequestTargetedMessageToKafka(
	adTagPubID string, requestID uuid.UUID, timestamp time.Time,
	geoCountry, deviceType string, publisherID uint64, requestType string, targetingID string, domain string,
	appName string, bundleID string,
) {
	msg := KafkaRequestMessageFormat{
		AdTagPubID:  adTagPubID,
		RequestID:   requestID.String(),
		Timestamp:   timestamp.Unix(),
		RequestType: requestType,
		GeoCountry:  geoCountry,
		DeviceType:  deviceType,
		PublisherID: publisherID,
		TargetingID: targetingID,
		Domain:      domain,
		AppName:     appName,
		BundleID:    bundleID,
	}

	msgJson, err := json.Marshal(msg)
	if err != nil {
		log.WithError(err).Warn("Can't marshall kafka message")
	}

	message := &sarama.ProducerMessage{
		Topic:     "requests_targeting",
		Partition: 0,
		Value:     sarama.StringEncoder(msgJson),
	}
	KafkaProducer.Input() <- message
}

func SendRTBEventMessageToKafka(requestContext request_context.RequestContext, eventName string, timestamp time.Time) {
	msg := message_format.KafkaRTBEventsMessageFormat{
		ID:             requestContext.RequestID.String(),
		PublisherID:    requestContext.PublisherID,
		TargetingID:    requestContext.PublisherTargetingID,
		EventName:      eventName,
		PublisherPrice: requestContext.PublisherPrice,
		Timestamp:      timestamp.Unix(),
		GeoCountry:     requestContext.User.Geo.Country.ISOCode,
		DeviceType:     requestContext.DevicePlatformType,
		Domain:         requestContext.Domain,
		AppName:        requestContext.AppName,
		BundleID:       requestContext.BundleID,
	}
	msgJson, err := json.Marshal(msg)
	if err != nil {
		log.WithError(err).Warn("Can't marshall kafka message")
	}

	message := &sarama.ProducerMessage{
		Topic:     "rtb_events",
		Partition: 0,
		Value:     sarama.StringEncoder(msgJson),
	}
	KafkaProducer.Input() <- message
}

func SendRTBBidRequestMessageToKafka(requestContext request_context.RequestContext, bidResponse BidResponseItem, timestamp time.Time) {

	var bidResponseReceived int8
	if bidResponse.BidEmpty == 0 && bidResponse.BidTimeout == 0 {
		bidResponseReceived = 1
	}

	msg := KafkaRTBBidRequestsMessageFormat{
		ID:             requestContext.RequestID.String(),
		PublisherID:    requestContext.PublisherID,
		AdvertiserID:   bidResponse.AdvertiserID,
		TargetingID:    requestContext.PublisherTargetingID,
		PublisherPrice: requestContext.PublisherPrice,
		Timestamp:      timestamp.Unix(),

		BidResponse:        bidResponseReceived,
		BidResponseTime:    bidResponse.TimeElapsed,
		BidResponseTimeout: bidResponse.BidTimeout,
		BidResponseEmpty:   bidResponse.BidEmpty,
		BidResponseError:   "", //TODO:
		BidWin:             bidResponse.BidWin,
		BidFloorPrice:      bidResponse.BidFloor,
		BidPrice:           bidResponse.BidPrice,
		SecondPrice:        bidResponse.SecondPrice,

		GeoCountry: requestContext.User.Geo.Country.ISOCode,
		DeviceType: requestContext.DevicePlatformType,
		Domain:     requestContext.Domain,
		AppName:    requestContext.AppName,
		BundleID:   requestContext.BundleID,
	}

	msgJson, err := json.Marshal(msg)
	if err != nil {
		log.WithError(err).Warn("Can't marshall kafka message")
	}

	message := &sarama.ProducerMessage{
		Topic:     "rtb_bid_requests",
		Partition: 0,
		Value:     sarama.StringEncoder(msgJson),
	}
	KafkaProducer.Input() <- message
}
