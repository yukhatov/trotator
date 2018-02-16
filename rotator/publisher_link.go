package rotator

import (
	"fmt"

	"bitbucket.org/tapgerine/traffic_rotator/rotator/data"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/redis_handler"
)

type PublisherLink struct {
	Data data.PublisherLinkData
}

func (p *PublisherLink) init(id string) error {
	data, err := data.ServingData.GetPublisherLinkDataByID(id)
	if err != nil {
		return err
	}
	p.Data = data
	return nil
}

func (p *PublisherLink) IsDomainAllowForThisLink(domain string) bool {
	if p.Data.DomainsListID > 0 {
		domainsListItem, err := redis_handler.RedisConnection.HGet(fmt.Sprintf("domains:%d", p.Data.DomainsListID), domain).Result()

		if p.Data.DomainsListType == "white" && !(err == nil && domainsListItem == "white") {
			// White list activated. Domain is not in the list
			return false
		} else if p.Data.DomainsListType == "black" && err == nil && domainsListItem == "black" {
			// Black list activated. Domain is in the list
			return false
		}
	}

	return true
}

func (p *PublisherLink) GetPublisherID() (uint64, error) {
	return data.ServingData.GetPublisherIDByTargetingID(p.Data.ID)
}

func (p *PublisherLink) GetAdTagIDs() ([]string, error) {
	return data.ServingData.GetAdTagIDsForPublisherLink(p.Data.ID)
}
