package vast

import (
	"fmt"
	"strconv"

	"encoding/json"

	"strings"

	"bitbucket.org/tapgerine/traffic_rotator/rotator/config"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/data"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/helpers"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/message_format"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/request_context"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/string_encryption"
	"github.com/bsm/openrtb"
)

const vastVPAIDTemplate = `<VAST version="%d.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="oxml.xsd">
	<Ad id="%s">
		<InLine>
			<AdSystem>Tapgerine</AdSystem>

			<AdTitle><![CDATA[Tapgerine]]></AdTitle>

			<Impression></Impression>

			<Creatives>
				<Creative>
					<Linear>
						<Duration><![CDATA[00:00:30]]></Duration>

						<AdParameters><![CDATA[%s]]></AdParameters>

						<MediaFiles>
							<MediaFile delivery="progressive" width="%d" height="%d" type="application/javascript" apiFramework="VPAID"><![CDATA[%s]]></MediaFile>
						</MediaFiles>
					</Linear>
				</Creative>
			</Creatives>

			<Extensions></Extensions>
		</InLine>
	</Ad>
</VAST>`

type AdParameter struct {
	URL           string `json:"url"`
	RequestUrl    string `json:"r"`
	ImpressionURL string `json:"i"`
}

func GenerateVASTVPAIDForOpenRTB(requestContext request_context.RequestContext) (string, error) {

	adParameters := fmt.Sprintf("https://%s%s", config.RotatorDomain, strings.Replace(requestContext.Request.RequestURI, "bidder_init", "bidder_processor", 1))
	//adParameters := fmt.Sprintf("https://%s%s", "412e2a95.ngrok.io", strings.Replace(requestContext.Request.RequestURI, "bidder_init", "bidder_processor", 1))

	return fmt.Sprintf(
		vastVPAIDTemplate,
		requestContext.VastVersion,
		requestContext.RequestID,
		adParameters, requestContext.Width, requestContext.Height, fmt.Sprintf("https://%s/static/js/vpaid_bidder.js", config.RotatorDomain),
		//adParameters, requestContext.Width, requestContext.Height, fmt.Sprintf("https://%s/static/js/vpaid_bidder.js", "11df6ede.ngrok.io"),
	), nil
}

func GenerateVASTVPAID(
	adTagsMap map[string]data.AdTagData, mappedURLs map[string]string, requestContext request_context.RequestContext,
	encryptionKey []byte,
) (string, error) {
	adParametersMap := make(map[int]AdParameter, len(mappedURLs))

	requestID := requestContext.RequestID.String()

	var tagsAdded int
	for adTagID, adTag := range adTagsMap {
		mappedUrl, exists := mappedURLs[adTagID]
		if exists {
			var price float64
			if requestContext.UseOriginPrice {
				price = adTag.Price
			} else {
				price = requestContext.PublisherPrice
			}

			impressionParams := EventParams{
				EventName:   "impression",
				RequestID:   requestID,
				AdTagPubID:  adTagID,
				Price:       price,
				RequestType: requestContext.Type,
				GeoCountry:  requestContext.User.Geo.Country.ISOCode,
				DeviceType:  requestContext.User.UserAgent.DeviceType,
				TargetingID: requestContext.PublisherTargetingID,
				Domain:      requestContext.Domain,
				AppName:     requestContext.AppName,
				BundleID:    requestContext.BundleID,
			}

			impressionParamsJson, err := json.Marshal(impressionParams)
			if err != nil {
				return "", err
			}

			impressionParamsEncrypted, err := stringEncryption.Encrypt(encryptionKey, impressionParamsJson)
			if err != nil {
				return "", err
			}

			requestParams := EventParams{
				EventName:   "request",
				RequestID:   requestID,
				AdTagPubID:  adTagID,
				RequestType: requestContext.Type,
				GeoCountry:  requestContext.User.Geo.Country.ISOCode,
				DeviceType:  requestContext.User.UserAgent.DeviceType,
				TargetingID: requestContext.PublisherTargetingID,
				Domain:      requestContext.Domain,
				AppName:     requestContext.AppName,
				BundleID:    requestContext.BundleID,
			}

			requestParamsJson, err := json.Marshal(requestParams)
			if err != nil {
				return "", err
			}

			requestParamsEncrypted, err := stringEncryption.Encrypt(encryptionKey, requestParamsJson)
			if err != nil {
				return "", err
			}

			adParametersMap[tagsAdded] = AdParameter{
				URL:           mappedUrl,
				ImpressionURL: fmt.Sprintf(`https://%s/events?data=%s`, config.StatsDomain, impressionParamsEncrypted),
				RequestUrl:    fmt.Sprintf(`https://%s/events?data=%s`, config.StatsDomain, requestParamsEncrypted),
			}
			tagsAdded++
		}
	}

	adParametersMapJson, err := json.Marshal(adParametersMap)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		vastVPAIDTemplate,
		requestContext.VastVersion, requestID, adParametersMapJson, requestContext.Width,
		requestContext.Height,
		fmt.Sprintf("https://%s/static/js/vpaid.js", config.RotatorDomain),
	), nil
}

type bidProcessorResponse struct {
	VastXML       string `json:"vast_xml"`
	WinUrl        string `json:"win_url"`
	ImpressionUrl string `json:"impression_url"`
	EventUrl      string `json:"event"`
}

func GenerateResponseForBiddingVPAID(
	bidResponse openrtb.BidResponse,
	advertiserID uint64,
	secondPrice float64,
	requestContext request_context.RequestContext,
	encryptionKey []byte,
) ([]byte, error) {

	markup := strings.Replace(
		bidResponse.SeatBid[0].Bid[0].AdMarkup,
		"${AUCTION_PRICE}",
		strconv.FormatFloat(secondPrice, 'f', 3, 64),
		1,
	)

	nurl := strings.Replace(
		bidResponse.SeatBid[0].Bid[0].NURL,
		"${AUCTION_PRICE}",
		strconv.FormatFloat(secondPrice, 'f', 3, 64),
		1,
	)

	impressionParams := EventParams{
		EventName:    "impression",
		RequestID:    requestContext.RequestID.String(),
		AdTagPubID:   "",
		Price:        secondPrice,
		RequestType:  "rtb",
		GeoCountry:   requestContext.User.Geo.Country.ISOCode,
		DeviceType:   requestContext.User.UserAgent.DeviceType,
		TargetingID:  requestContext.PublisherTargetingID,
		Domain:       requestContext.Domain,
		AppName:      requestContext.AppName,
		BundleID:     requestContext.BundleID,
		AdvertiserID: advertiserID,
		PublisherID:  requestContext.PublisherID,
		OriginPrice:  requestContext.PublisherPrice,
	}

	impressionParamsJson, err := json.Marshal(impressionParams)
	if err != nil {
		return nil, err
	}

	impressionParamsEncrypted, err := stringEncryption.Encrypt(encryptionKey, impressionParamsJson)
	if err != nil {
		return nil, err
	}

	rtbEventParams := message_format.KafkaRTBEventsMessageFormat{
		ID:             requestContext.RequestID.String(),
		PublisherID:    requestContext.PublisherID,
		TargetingID:    requestContext.PublisherTargetingID,
		PublisherPrice: requestContext.PublisherPrice,
		GeoCountry:     requestContext.User.Geo.Country.ISOCode,
		DeviceType:     requestContext.DevicePlatformType,
		Domain:         requestContext.Domain,
		AppName:        requestContext.AppName,
		BundleID:       requestContext.BundleID,
	}

	rtbEventParamsJson, err := json.Marshal(rtbEventParams)
	if err != nil {
		return nil, err
	}

	rtbEventParamsEncrypted, err := stringEncryption.Encrypt(encryptionKey, rtbEventParamsJson)
	if err != nil {
		return nil, err
	}

	response, err := helpers.JSONMarshalWithoutEncoding(bidProcessorResponse{
		VastXML:       markup,
		WinUrl:        nurl,
		ImpressionUrl: fmt.Sprintf(`https://%s/events?data=%s`, config.StatsDomain, impressionParamsEncrypted),
		EventUrl:      fmt.Sprintf(`https://%s/r_events?e={e}&data=%s`, config.StatsDomain, rtbEventParamsEncrypted),
	})

	return response, err
}
