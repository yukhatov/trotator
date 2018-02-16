package rotator

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"bitbucket.org/tapgerine/traffic_rotator/rotator/request_context"
	uuid "github.com/satori/go.uuid"
)

var (
	ErrPriceParsing = errors.New("price parsing error")
)

func parseRequest(r *http.Request) (request_context.RequestContext, error) {
	var err error
	context := &request_context.RequestContext{
		Request:   r,
		Type:      "targeting",
		RequestID: uuid.NewV4(),
		User:      request_context.UserContext{},
		Width:     640,
		Height:    360,
	}

	switch r.URL.Query().Get("response") {
	case "vast20vpaid":
		context.ResponseType = "vpaid"
		context.Type = "vpaid"
		context.VastVersion = 2
	case "vast30vpaid":
		context.ResponseType = "vpaid"
		context.Type = "vpaid"
		context.VastVersion = 3
	case "vast20wrapper":
		context.ResponseType = "vast"
		context.VastVersion = 2
	case "vast30wrapper":
		context.ResponseType = "vast"
		context.VastVersion = 3
	default:
		context.VastVersion = 2
	}

	context.PublisherTargetingID = r.URL.Query().Get("pub")

	// TODO: this is hack for stupid pub
	if context.PublisherTargetingID == "pipxcjax" {
		context.PublisherTargetingID = "PIpxcjaX"
	}

	if context.PublisherTargetingID == "" {
		return *context, errors.New("no publisher targeting id")
	}

	p := r.URL.Query().Get("price")

	if p == "origin" {
		context.UseOriginPrice = true
	} else {
		p = strings.Trim(p, "/")            // This is because of stupid publisher
		p = strings.Replace(p, ",", ".", 1) // This is because of second stupid publisher
		context.PublisherPrice, err = strconv.ParseFloat(p, 64)
		if err != nil {
			context.PriceParsingError = ErrPriceParsing
		}
	}

	// TODO: we need scheme to parse request parameters
	context.ParseIP(r.URL.Query().Get("ip"), r)
	//context.User.IP = net.ParseIP(r.URL.Query().Get("ip"))
	err = context.ParseGeo()
	if err != nil {
		return *context, err
	}

	context.User.UserAgentString = r.URL.Query().Get("ua")
	if context.User.UserAgentString == "" || context.User.UserAgentString == "[USER_AGENT]" {
		headerUA := r.Header.Get("User-Agent")
		if headerUA != "" {
			context.User.UserAgentString = headerUA
		} else {
			context.User.UserAgentString = ""
		}
	}
	context.ParseUserAgent()

	w := r.URL.Query().Get("w")
	width, err := strconv.Atoi(w)
	if err == nil && width > 0 {
		context.Width = width
	}

	h := r.URL.Query().Get("h")
	height, err := strconv.Atoi(h)
	if err == nil && height > 0 {
		context.Height = height
	}

	context.ParseDomain(r.URL.Query().Get("url"), r)

	appName := r.URL.Query().Get("appname")
	if appName != "[APP_NAME]" {
		context.AppName = appName
	}

	bundleID := r.URL.Query().Get("bundle_id")
	if bundleID != "[BUNDLE_ID]" {
		context.BundleID = bundleID
	}

	appStoreURL := r.URL.Query().Get("appstoreurl")
	if bundleID != "[APP_STORE_URL]" {
		context.AppStoreURL = appStoreURL
	}

	dnt := r.URL.Query().Get("dnt")

	if dnt != "[DO_NOT_TRACK]" && (dnt == "1" || dnt == "true") {
		context.DoNotTrack = 1
	}

	return *context, nil
}
