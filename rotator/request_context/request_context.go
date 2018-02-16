package request_context

import (
	"net"
	"net/http"

	"net/url"
	"strings"

	"fmt"

	"github.com/mssola/user_agent"
	uuid "github.com/satori/go.uuid"
)

// RequestContext describes request after parsing
type RequestContext struct {
	Request              *http.Request
	Type                 string
	RequestID            uuid.UUID
	User                 UserContext
	PublisherID          uint64
	PublisherTargetingID string
	PublisherPrice       float64
	PriceParsingError    error
	UseOriginPrice       bool
	ResponseType         string
	Width                int
	Height               int
	Domain               string
	Referrer             string
	RequestPlatform      string
	AppName              string
	BundleID             string
	AppStoreURL          string
	DevicePlatformType   string
	VastVersion          int
	DoNotTrack           int
}

type UserContext struct {
	IP              net.IP
	Geo             Country
	UserAgent       UserAgent
	UserAgentString string
}

type Country struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
		//GeoNameID uint              `maxminddb:"geoname_id"`
		//Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}

type UserAgent struct {
	IsMobile   bool
	IsBot      bool
	Browser    UserAgentBrowser
	OS         UserAgentOS
	DeviceType string
}

type UserAgentBrowser struct {
	Name string
}

type UserAgentOS struct {
	Name string
}

func (r *RequestContext) ParseIP(ipFromRequest string, req *http.Request) {
	ipFromRequest = strings.TrimSpace(ipFromRequest)
	if ipFromRequest != "" {
		r.User.IP = net.ParseIP(ipFromRequest)
	} else if req.Header.Get("X-Forwarded-For") != "" {
		ip := getIPAddress(req)
		if ip != "" {
			r.User.IP = net.ParseIP(ip)
		}
	} else {
		ip, _, err := net.SplitHostPort(req.RemoteAddr)
		if err == nil {
			r.User.IP = net.ParseIP(ip)
		}
	}
}

func (r *RequestContext) ParseGeo() error {
	err := GeoDatabase.Lookup(r.User.IP, &r.User.Geo)
	if err != nil {
		return err
	}
	return nil
}

func (r *RequestContext) ParseUserAgent() {
	if r.User.UserAgentString == "" {
		r.User.UserAgent.DeviceType = "undefined"
		return
	}

	ua := user_agent.New(r.User.UserAgentString)

	r.User.UserAgent.IsMobile = ua.Mobile()
	if r.User.UserAgent.IsMobile {
		r.User.UserAgent.DeviceType = "mobile"
	} else {
		r.User.UserAgent.DeviceType = "desktop"
	}

	r.User.UserAgent.IsBot = ua.Bot()
	r.User.UserAgent.OS.Name = ua.OS()
	r.User.UserAgent.Browser.Name, _ = ua.Browser()
}

func (r *RequestContext) ParseDomain(pageURL string, req *http.Request) {
	if pageURL == "" || pageURL == "[PAGE_URL]" {
		pageURL = req.Header.Get("Referer")
	}

	if !strings.Contains(pageURL, ".") {
		return
	}

	r.Referrer = pageURL

	if pageURL != "" {
		pageURL = strings.ToLower(pageURL)
		pageURL = strings.TrimSpace(pageURL)
		pageURL = strings.Replace(pageURL, "www.", "", 1)
	}

	parsed, err := url.Parse(pageURL)
	if err == nil && parsed.Scheme == "" {
		pageURL = fmt.Sprintf("http://%s", pageURL)
		parsed, err = url.Parse(pageURL)
	}

	if err != nil {
		pageURL = strings.Replace(pageURL, "%", "", -1)
		parsed, err = url.Parse(pageURL)
		if err == nil {
			r.Domain = parsed.Hostname()
		}
	} else {
		r.Domain = parsed.Hostname()
	}
}

func (r *RequestContext) SetRequestPlatform(platform string) {
	r.RequestPlatform = platform

	switch r.RequestPlatform {
	case "desktop":
		r.DevicePlatformType = r.User.UserAgent.DeviceType
	case "in-app":
		r.DevicePlatformType = "in-app"
	}
}
