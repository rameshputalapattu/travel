// Package advisory provides support for managing advisory data in the database.
package advisory

import (
	"context"
	"fmt"
	"log"

	"github.com/ardanlabs/graphql"
	"github.com/dgraph-io/travel/business/data"
	"github.com/pkg/errors"
)

// Set of error variables for CRUD operations.
var (
	ErrNotFound = errors.New("advisory not found")
)

// Advisory manages the set of API's for advisory access.
type Advisory struct {
	log *log.Logger
	gql *graphql.GraphQL
}

// New constructs a Advisory for api access.
func New(log *log.Logger, gql *graphql.GraphQL) Advisory {
	return Advisory{
		log: log,
		gql: gql,
	}
}

// Replace replaces an advisory in the database and connects it
// to the specified city.
func (a Advisory) Replace(ctx context.Context, traceID string, adv Info) (Info, error) {
	if adv.ID != "" {
		return Info{}, errors.New("advisory contains id")
	}
	if adv.City.ID == "" {
		return Info{}, errors.New("cityid not provided")
	}

	if oldAdv, err := a.QueryByCity(ctx, traceID, adv.City.ID); err == nil {
		if err := a.delete(ctx, traceID, oldAdv.ID); err != nil {
			if err != ErrNotFound {
				return Info{}, errors.Wrap(err, "deleting advisory from database")
			}
		}
	}

	return a.add(ctx, traceID, adv)
}

// QueryByCity returns the specified advisory from the database by the city id.
func (a Advisory) QueryByCity(ctx context.Context, traceID string, cityID string) (Info, error) {
	query := fmt.Sprintf(`
query {
	getCity(id: %q) {
		advisory {
			id
			city {
				id
			}
			continent
			country
			country_code
			last_updated
			message
			score
			source
		}
	}
}`, cityID)

	a.log.Printf("%s: %s: %s", traceID, "advisory.QueryByID", data.Log(query))

	var result struct {
		GetCity struct {
			Advisory Info `json:"advisory"`
		} `json:"getCity"`
	}
	if err := a.gql.Query(ctx, query, &result); err != nil {
		return Info{}, errors.Wrap(err, "query failed")
	}

	if result.GetCity.Advisory.ID == "" {
		return Info{}, ErrNotFound
	}

	return result.GetCity.Advisory, nil
}

// =============================================================================

func (a Advisory) add(ctx context.Context, traceID string, adv Info) (Info, error) {
	var result id
	mutation := fmt.Sprintf(`
	mutation {
		resp: addAdvisory(input: [{
			city: {
				id: %q
			}
			continent: %q
			country: %q
			country_code: %q
			last_updated: %q
			message: %q
			score: %f
			source: %q
		}])
		%s
	}`, adv.City.ID, adv.Continent, adv.Country, adv.CountryCode,
		adv.LastUpdated, adv.Message, adv.Score, adv.Source,
		result.document())

	a.log.Printf("%s: %s: %s", traceID, "advisory.Add", data.Log(mutation))

	if err := a.gql.Query(ctx, mutation, &result); err != nil {
		return Info{}, errors.Wrap(err, "failed to add place")
	}

	if len(result.Resp.Entities) != 1 {
		return Info{}, errors.New("advisory id not returned")
	}

	adv.ID = result.Resp.Entities[0].ID
	return adv, nil
}

func (a Advisory) delete(ctx context.Context, traceID string, advID string) error {
	var result result
	mutation := fmt.Sprintf(`
	mutation {
		resp: deleteAdvisory(filter: { id: [%q] })
		%s
	}`, advID, result.document())

	a.log.Printf("%s: %s: %s", traceID, "advisory.Delete", data.Log(mutation))

	if err := a.gql.Query(ctx, mutation, &result); err != nil {
		return errors.Wrap(err, "failed to delete advisory")
	}

	if result.Resp.NumUids != 1 {
		msg := fmt.Sprintf("failed to delete advisory: NumUids: %d  Msg: %s", result.Resp.NumUids, result.Resp.Msg)
		return errors.New(msg)
	}

	return nil
}
