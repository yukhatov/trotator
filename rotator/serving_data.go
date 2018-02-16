package rotator

//
//import (
//	"encoding/json"
//	"errors"
//	"fmt"
//	"sync"
//	"time"
//
//	log "github.com/Sirupsen/logrus"
//)
//
////var DataWriteLock sync.RWMutex
//var ServingData *ParsedServingData
//
//type SyncData struct {
//	AdTags                       map[string]AdTagData                    `json:"ad_tags"`
//	ParametersMapping            map[uint64]map[string]ParametersMapping `json:"parameters_mapping"`
//	OurPlatformParametersMapping map[string]map[uint64]ParametersMapping `json:"our_platform_parameters_mapping"`
//	PublisherTargetingIDMap      map[string]uint64                       `json:"publisher_targeting_id_map"`
//}
//
//type AdTagData struct {
//	AdTagID                  uint64                 `json:"id"`
//	URL                      string                 `json:"url"`
//	SupportsVast             bool                   `json:"supports_vast"`
//	IsActive                 bool                   `json:"is_active"`
//	IsAdTagPubActive         bool                   `json:"is_ad_tag_pub_active"`
//	IsTest                   bool                   `json:"is_test"`
//	AdvertiserPlatformTypeID uint64                 `json:"advertiser_platform_type_id"`
//	Targeting                AdTagTargeting         `json:"targeting"`
//	PublisherID              uint64                 `json:"publisher_id"`
//	PublisherTargetingID     string                 `json:"publisher_targeting_id"`
//	Price                    float64                `json:"price"`
//	CouldBeUsedForTargeting  bool                   `json:"could_be_used_for_targeting"`
//	ERPRByGeoForLastWeek     map[string]ERPRByGeo   `json:"erpr_by_geo_for_last_week"`
//	TotalStats               TotalStatsForLastMonth `json:"total_stats"`
//}
//
//type AdTagTargeting struct {
//	Geo        []string `json:"geo_targeting"`
//	DeviceType string   `json:"device_type"`
//}
//
//type ParametersMapping struct {
//	Shortcut         string `json:"shortcut"`
//	Macros           string `json:"macros"`
//	OriginalShortcut string `json:"original_shortcut"`
//	OriginalMacros   string `json:"original_macros"`
//}
//
//type ERPRByGeo struct {
//	Requests    int64   `json:"requests"`
//	Impressions int64   `json:"impressions"`
//	FillRate    float64 `json:"fill_rate"`
//	Margin      float64 `json:"margin"`
//}
//
//type TotalStatsForLastMonth struct {
//	Impressions int64
//	Requests    int64
//}
//
//type ParsedServingData struct {
//	Data          SyncData
//	AdTagKeys     []string
//	Expiration    int64
//	IsInitialized bool
//	DataWriteLock sync.RWMutex
//}
//
//func (p *ParsedServingData) IsExpired() bool {
//	return time.Now().UTC().Unix() >= p.Expiration
//}
//
//func (p *ParsedServingData) GetNewExpirationTime() int64 {
//	// Default expiration is 59 sec
//	return time.Now().UTC().Unix() + 60
//}
//
//func (p *ParsedServingData) CheckData() error {
//	p.DataWriteLock.Lock()
//	defer p.DataWriteLock.Unlock()
//	if !p.IsInitialized || p.IsExpired() {
//		data, err := RedisConnection.Get("serving_data").Bytes()
//		if err != nil {
//			log.Warn("No data in redis")
//			return err
//		}
//		p.Data = SyncData{}
//		json.Unmarshal(data, &p.Data)
//		p.Expiration = p.GetNewExpirationTime()
//		p.IsInitialized = true
//
//		//i := 0
//		//p.AdTagKeys = make([]string, len(p.Data))
//		//for key := range p.Data {
//		//	p.AdTagKeys[i] = key
//		//	i++
//		//}
//	}
//
//	return nil
//}
//
//func (p *ParsedServingData) GetAdTagByID(id string) (AdTagData, error) {
//	err := p.CheckData()
//	if err != nil {
//		return AdTagData{}, err
//	}
//
//	p.DataWriteLock.RLock()
//	adTag, exists := p.Data.AdTags[id]
//	p.DataWriteLock.RUnlock()
//
//	if exists {
//		return adTag, nil
//	} else {
//		//log.Warn(fmt.Sprintf("No data for ad tag pub id %s", id))
//		return AdTagData{}, errors.New("no data")
//	}
//}
//
//func (p *ParsedServingData) GetAllAdTags() (map[string]AdTagData, error) {
//	err := p.CheckData()
//	if err != nil {
//		return map[string]AdTagData{}, err
//	}
//	return p.Data.AdTags, nil
//}
//
//func (p *ParsedServingData) GetAdTagsByIDs(ids []string) (map[string]AdTagData, error) {
//	var result map[string]AdTagData
//	var err error
//
//	err = p.CheckData()
//	if err != nil {
//		return result, err
//	}
//
//	result = make(map[string]AdTagData, len(ids))
//
//	var adTag AdTagData
//	for _, id := range ids {
//		adTag, err = p.GetAdTagByID(id)
//		if err == nil {
//			result[id] = adTag
//		}
//	}
//
//	return result, nil
//}
//
//func (p *ParsedServingData) GetParametersMapByID(id uint64) (map[string]ParametersMapping, error) {
//	var result map[string]ParametersMapping
//
//	err := p.CheckData()
//	if err != nil {
//		return result, err
//	}
//
//	p.DataWriteLock.RLock()
//	var exists bool
//	result, exists = p.Data.ParametersMapping[id]
//	p.DataWriteLock.RUnlock()
//
//	if !exists {
//		log.Warn(fmt.Sprintf("No data for parameter id %d", id))
//		return result, errors.New("no data")
//	}
//
//	return result, nil
//}
//
//func (p *ParsedServingData) GetOurPlatformParametersMapByNameAndID(parameterName string, id uint64) (ParametersMapping, error) {
//	var result ParametersMapping
//
//	err := p.CheckData()
//	if err != nil {
//		return result, err
//	}
//
//	var exists bool
//
//	p.DataWriteLock.RLock()
//	_, exists = p.Data.OurPlatformParametersMapping[parameterName]
//	p.DataWriteLock.RUnlock()
//
//	if !exists {
//		//log.Warn(fmt.Sprintf("No data for parameter name %s", parameterName))
//		return result, errors.New("no data")
//	}
//
//	p.DataWriteLock.RLock()
//	result, exists = p.Data.OurPlatformParametersMapping[parameterName][id]
//	p.DataWriteLock.RUnlock()
//
//	if !exists {
//		//log.Warn(fmt.Sprintf("No data for parameter name %s and platform id %d", parameterName, id))
//		return result, errors.New("no data")
//	}
//
//	return result, nil
//}
//
//func (p *ParsedServingData) GetPublisherIDByTargetingID(targetingID string) (uint64, error) {
//	var result uint64
//	err := p.CheckData()
//	if err != nil {
//		return result, err
//	}
//
//	p.DataWriteLock.RLock()
//	var exists bool
//	result, exists = p.Data.PublisherTargetingIDMap[targetingID]
//	p.DataWriteLock.RUnlock()
//
//	if !exists {
//		log.Warn(fmt.Sprintf("No data for targeting id %s", targetingID))
//		return result, errors.New("no data")
//	}
//
//	return result, nil
//}
