package rotator

import (
	"bytes"
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	"io/ioutil"

	"bitbucket.org/tapgerine/traffic_rotator/rotator/data"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/request_context"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/vast"
	log "github.com/Sirupsen/logrus"
	"github.com/bsm/openrtb"
)

var bidResponseTimeout = []byte("timeout")
var bidResponseEmpty = []byte("empty")

type BidResponseMetadata struct {
	BidResponseJSON []byte
	AdvertiserID    uint64
	TimeElapsed     time.Duration
}
type BidResponseItem struct {
	BidResponse   openrtb.BidResponse
	AdvertiserID  uint64
	BidFloor      float64
	BidFloorError bool
	BidPrice      float64
	SecondPrice   float64
	BidTimeout    int8
	BidEmpty      int8
	BidWin        int8
	TimeElapsed   int64
}

func AdRotationOpenRTBProcessorHandler(w http.ResponseWriter, r *http.Request) {
	timestamp := time.Now().UTC()
	requestContext, err := parseRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		log.WithField("url", r.URL.String()).Warn(err)
		return
	}

	var publisherLink PublisherLink
	if err = publisherLink.init(requestContext.PublisherTargetingID); err != nil {
		w.WriteHeader(http.StatusNoContent)
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

	SendRTBEventMessageToKafka(requestContext, "auction", timestamp)

	bidFloor := requestContext.PublisherPrice + 0.5
	bidRequest := composeBidRequest(&requestContext, bidFloor)

	bidRequestJSON, err := json.Marshal(bidRequest)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		log.WithField("url", r.URL.String()).WithError(err).Warn()
		return
	}

	dspList, err := data.ServingData.GetDSPAdvertisersList()
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		log.WithField("url", r.URL.String()).WithError(err).Warn()
		return
	}

	bidResponsesChannel := make(chan BidResponseMetadata, len(dspList))

	for _, dsp := range dspList {
		go makeBidRequest(dsp.RTBIntegrationUrl, bidRequestJSON, dsp.ID, bidResponsesChannel)
		//go makeBidRequest("http://localhost:8082/http_test", bidRequestJSON, dsp.ID, bidResponsesChannel)
	}

	validBidResponsesCount, bidResponseList, maxBidIndex, secondMaxBid := collectBidResponses(len(dspList), bidFloor, bidResponsesChannel)

	var bidWinSecondPrice float64
	var bidWin BidResponseItem

	if validBidResponsesCount > 0 {
		bidWin = bidResponseList[maxBidIndex]
		if secondMaxBid > 0 {
			bidWinSecondPrice = secondMaxBid + 0.01
			if bidWinSecondPrice > bidWin.BidResponse.SeatBid[0].Bid[0].Price {
				bidWinSecondPrice = bidWin.BidResponse.SeatBid[0].Bid[0].Price
			}
		} else {
			//bidWinSecondPrice = bidWin.BidResponse.SeatBid[0].Bid[0].Price
			minimumPrice := bidFloor + 0.01
			maximumPrice := bidWin.BidResponse.SeatBid[0].Bid[0].Price
			bidWinSecondPrice = rand.Float64()*(maximumPrice-minimumPrice) + minimumPrice
		}

		bidResponseList[maxBidIndex].BidWin = 1
		bidResponseList[maxBidIndex].SecondPrice = bidWinSecondPrice
	}

	for _, bidResponse := range bidResponseList {
		SendRTBBidRequestMessageToKafka(requestContext, bidResponse, timestamp)
	}

	if validBidResponsesCount == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	response, err := vast.GenerateResponseForBiddingVPAID(
		bidWin.BidResponse,
		bidWin.AdvertiserID,
		bidWinSecondPrice,
		requestContext,
		EncryptionKey,
	)

	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		log.WithField("url", r.URL.String()).Warn(err)
		return
	}

	w.Header().Set("Content-type", "application/json")
	//w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(response)
	return
}

func collectBidResponses(bidRequestsCount int, bidFloor float64, bidResponsesChannel <-chan BidResponseMetadata) (int, []BidResponseItem, int, float64) {
	var bidResponseList []BidResponseItem
	bidResponseList = make([]BidResponseItem, bidRequestsCount)

	var maxBid, secondMaxBid float64
	var maxBidIndex int
	var bidPrice float64
	var bidResponse openrtb.BidResponse
	var bidFloorError bool
	var bidResponseValid bool
	var validBidResponsesCount int

	for i := 0; i < bidRequestsCount; i++ {
		bidFloorError = false
		b := <-bidResponsesChannel
		isTimeout := bytes.Equal(b.BidResponseJSON, bidResponseTimeout)
		isEmpty := bytes.Equal(b.BidResponseJSON, bidResponseEmpty)
		bidResponseValid = !isTimeout && !isEmpty

		if bidResponseValid {
			// TODO: add logic for finding correct bid responses
			err := json.Unmarshal(b.BidResponseJSON, &bidResponse)
			if err != nil {
				log.WithField("json", string(b.BidResponseJSON)).WithError(err).Warn()
				continue
			}
			log.Info(string(b.BidResponseJSON))
			//fmt.Println("=======================")
			//fmt.Println(string(b.BidResponseJSON))
			//fmt.Println("=======================")

			bidPrice = bidResponse.SeatBid[0].Bid[0].Price

			if bidFloor > bidPrice {
				bidFloorError = true
			} else {
				if bidPrice > maxBid {
					secondMaxBid = maxBid
					maxBid = bidPrice
					maxBidIndex = i
				}
				validBidResponsesCount++
			}
		}

		bidResponseList[i] = BidResponseItem{
			BidResponse:   bidResponse,
			AdvertiserID:  b.AdvertiserID,
			BidFloorError: bidFloorError,
			BidPrice:      bidPrice,
			BidFloor:      bidFloor,
			TimeElapsed:   int64(time.Duration(b.TimeElapsed.Nanoseconds()) / time.Millisecond),
		}

		if isTimeout {
			bidResponseList[i].BidTimeout = 1
		}
		if isEmpty {
			bidResponseList[i].BidEmpty = 1
		}
	}

	return validBidResponsesCount, bidResponseList, maxBidIndex, secondMaxBid
}

func makeBidRequest(url_ string, bidRequest []byte, advertiserID uint64, ch chan<- BidResponseMetadata) {
	ctx, _ := context.WithTimeout(context.Background(), 300*time.Millisecond)
	req, _ := http.NewRequest("POST", url_, bytes.NewBuffer(bidRequest))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-openrtb-version", "2.5")

	//t := []byte(`{"id":"55b341f6-ad96-4915-be72-b211eb33a9a5","seatbid":[{"bid":[{"id":"-8lm-EcieV4_0","impid":"1","price":5.6,"adid":"113107","adm":"<?xml version='1.0' ?><VAST version='2.0'><Ad id='113107'><Wrapper><AdSystem version='0.1'>AdKernel</AdSystem><VASTAdTagURI><![CDATA[https://ssp.streamrail.net/ssp/vpaid/5a03fac337a0fc0002000001/5a05b8b64d55cf000270a97f?cb=971042983\u0026width=300\u0026height=250\u0026video_duration=\u0026video_description=\u0026video_url=\u0026video_id=\u0026video_title=\u0026pos=\u0026autoplay=\u0026mute=\u0026page_url=http%3A%2F%2Ftheatlantic.com]]></VASTAdTagURI><Error><![CDATA[https://rtb.readywind.com/vast-error?i=-8lm-EcieV4_0\u0026err=[ERRORCODE]]]></Error><Impression><![CDATA[https://rtb.readywind.com/win?i=-8lm-EcieV4_0\u0026price=${AUCTION_PRICE}\u0026f=imp]]></Impression><Creatives><Creative><Linear><TrackingEvents><Tracking event='firstQuartile'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=first_quartile]]></Tracking><Tracking event='midpoint'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=midpoint]]></Tracking><Tracking event='thirdQuartile'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=third_quartile]]></Tracking><Tracking event='complete'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=complete]]></Tracking><Tracking event='mute'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=mute]]></Tracking><Tracking event='unmute'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=unmute]]></Tracking><Tracking event='pause'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=pause]]></Tracking><Tracking event='resume'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=resume]]></Tracking><Tracking event='fullscreen'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=fullscreen]]></Tracking><Tracking event='close'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=close]]></Tracking><Tracking event='acceptInvitation'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=accept_invitation]]></Tracking><Tracking event='skip'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=skip]]></Tracking></TrackingEvents><VideoClicks><ClickTracking><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=click]]></ClickTracking></VideoClicks></Linear></Creative><Creative><NonLinearAds><TrackingEvents><Tracking event='expand'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=overlay_expand]]></Tracking><Tracking event='collapse'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=overlay_collapse]]></Tracking><Tracking event='acceptInvitation'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=overlay_accept_invitation]]></Tracking><Tracking event='close'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=overlay_close]]></Tracking><Tracking event='complete'><![CDATA[https://rtb.readywind.com/vast-track?i=-8lm-EcieV4_0\u0026ve=overlay_complete]]></Tracking></TrackingEvents></NonLinearAds></Creative></Creatives></Wrapper></Ad></VAST>","cid":"113107","crid":"113107"}],"seat":"r113107"}],"cur":"USD"}`)

	var metadata BidResponseMetadata
	metadata.AdvertiserID = advertiserID
	startTime := time.Now()

	//if rand.Intn(100) < 10 {
	//	metadata.TimeElapsed = time.Now().Sub(startTime)
	//	resp := fmt.Sprintf(`{"id":"55b341f6-ad96-4915-be72-b211eb33a9a5","seatbid":[{"bid":[{"id":"-8lm-EcieV4_0","impid":"1","price":5.6,"adid":"113107","adm":"<?xml version=\"1.0\" encoding=\"UTF-8\"?><VAST version=\"2.0\"><Ad id=\"113107\"><Wrapper><AdSystem version=\"0.1\">AdKernel</AdSystem><VASTAdTagURI><![CDATA[http://vid.springserve.com/vast/215294?w=%d&h=%d&url=%s&cb=[CACHEBUSTER]&ip=%s&ua=%s]]></VASTAdTagURI><Creatives><Creative><Linear><TrackingEvents /><VideoClicks><ClickTracking /></VideoClicks></Linear></Creative><Creative><NonLinearAds /></Creative></Creatives></Wrapper></Ad></VAST>","cid":"113107","crid":"113107"}],"seat":"r113107"}],"cur":"USD"}`, requestContext.Width, requestContext.Height, url.QueryEscape(requestContext.Referrer), requestContext.User.IP.String(), url.QueryEscape(requestContext.User.UserAgentString))
	//	metadata.BidResponseJSON = []byte(resp)
	//	ch <- metadata
	//	return
	//}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		metadata.TimeElapsed = time.Now().Sub(startTime)
		metadata.BidResponseJSON = bidResponseTimeout
		ch <- metadata
	} else {
		metadata.TimeElapsed = time.Now().Sub(startTime)
		if resp.StatusCode == 204 || resp.ContentLength == 0 {
			log.WithField("headers", resp.Header).Info(resp.StatusCode)
			metadata.BidResponseJSON = bidResponseEmpty
			ch <- metadata
			resp.Body.Close()
			return
		}
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		metadata.BidResponseJSON = body

		//metadata.BidResponseJSON = t

		ch <- metadata
	}
}

func composeBidRequest(requestContext *request_context.RequestContext, bidFloor float64) openrtb.BidRequest {
	bidRequest := openrtb.BidRequest{
		ID:          requestContext.RequestID.String(),
		Cur:         []string{"USD"},
		TMax:        300,
		AuctionType: 2,
		Imp: []openrtb.Impression{
			{
				ID: "1",
				Video: &openrtb.Video{
					Mimes:       []string{"video/mp4"},
					W:           requestContext.Width,
					H:           requestContext.Height,
					StartDelay:  0,
					Protocols:   []int{2, 3, 5, 6},
					Sequence:    1,
					Linearity:   1,
					MinDuration: 1,
					MaxDuration: 3600,
					Api:         []int{1, 2},
				},
				BidFloorCurrency: "USD",
				BidFloor:         bidFloor,
				Secure:           1, // TODO: check incoming schema
			},
		},
		Device: &openrtb.Device{
			IP: requestContext.User.IP.String(),
			UA: requestContext.User.UserAgentString,
			Geo: &openrtb.Geo{
				Country: requestContext.User.Geo.Country.ISOCode,
			},
			DNT: requestContext.DoNotTrack,
			JS:  1,
		},
		User: &openrtb.User{
			Geo: &openrtb.Geo{
				Country: requestContext.User.Geo.Country.ISOCode,
			},
		},
	}

	if requestContext.DevicePlatformType == "in-app" {
		bidRequest.App = &openrtb.App{
			Inventory: openrtb.Inventory{
				Name: requestContext.AppName,
			},
			Bundle:   requestContext.BundleID,
			StoreURL: requestContext.AppStoreURL,
		}
	} else {
		bidRequest.Site = &openrtb.Site{
			Inventory: openrtb.Inventory{
				Domain: requestContext.Domain,
			},
			Page: requestContext.Referrer,
		}
	}

	return bidRequest
}
