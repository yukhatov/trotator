package vast

import (
	"encoding/json"
	"fmt"

	"bitbucket.org/tapgerine/traffic_rotator/rotator/config"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/string_encryption"
)

const vastWrapperTemplate = `<VAST version="%d.0">
	<Ad id="%d">
		<Wrapper>
			<AdSystem>Tapgerine</AdSystem>
			<VASTAdTagURI>
				<![CDATA[%s]]>
			</VASTAdTagURI>
			<Impression>%s</Impression>
			<Creatives>
				<Creative AdID="%d">
					<Linear>
						<TrackingEvents>
						</TrackingEvents>
					</Linear>
				</Creative>
			</Creatives>
		</Wrapper>
	</Ad>
</VAST>`

//var EventsURL = fmt.Sprintf(`https://%s/events?data=%%s`, config.StatsDomain)

type EventParams struct {
	EventName    string  `json:"e"`
	RequestID    string  `json:"r"`
	AdTagPubID   string  `json:"atpid"`
	Price        float64 `json:"p"`
	RequestType  string  `json:"r_type"`
	GeoCountry   string  `json:"geo"`
	DeviceType   string  `json:"d"`
	TargetingID  string  `json:"tid"`
	Domain       string  `json:"domain"`
	AppName      string  `json:"app"`
	BundleID     string  `json:"bid"`
	Timestamp    int64   `json:"t"`
	AdvertiserID uint64  `json:"a_id"`
	PublisherID  uint64  `json:"p_id"`
	OriginPrice  float64 `json:"or_price"`
}

func GenerateVASTWrapper(
	adTagID uint64, repackedURL, requestID, adTagPubID string, price float64, encryptionKey []byte,
	requestType, geoCountry, deviceType string, targetingID string, domain string, appName string, bundleID string,
	vastVersion int,
) (string, error) {
	// TODO: make url constant?
	//impressionURL := "https://pmp-stats.tapgerine.com/events?data=%s"
	params := EventParams{
		EventName:   "impression",
		RequestID:   requestID,
		AdTagPubID:  adTagPubID,
		Price:       price,
		RequestType: requestType,
		GeoCountry:  geoCountry,
		DeviceType:  deviceType,
		TargetingID: targetingID,
		Domain:      domain,
		AppName:     appName,
		BundleID:    bundleID,
	}

	jsonParams, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	encryptedParams, err := stringEncryption.Encrypt(encryptionKey, jsonParams)
	if err != nil {
		return "", err
	}

	impressionURL := fmt.Sprintf(`https://%s/events?data=%s`, config.StatsDomain, encryptedParams)

	return fmt.Sprintf(vastWrapperTemplate, vastVersion, adTagID, repackedURL, impressionURL, adTagID), nil
}
