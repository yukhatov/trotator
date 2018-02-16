package message_format

type KafkaRTBEventsMessageFormat struct {
	ID             string  `json:"id"`
	PublisherID    uint64  `json:"pid"`
	TargetingID    string  `json:"tid"`
	EventName      string  `json:"e"`
	PublisherPrice float64 `json:"price"`
	Timestamp      int64   `json:"timestamp"`
	GeoCountry     string  `json:"geo_country"`
	DeviceType     string  `json:"device_type"`
	Domain         string  `json:"domain"`
	AppName        string  `json:"app_name"`
	BundleID       string  `json:"bundle_id"`
}
