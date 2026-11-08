// Package publishedoffers reads the coding plan offers the maintainer
// publishes in the public repository (SPECS §12, ADR 0017):
// `data/offers.csv` on the main branch, fetched from Codeberg without any
// key. A file not published yet (404) is no offer.
package publishedoffers

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/simplicer/ataecina/internal/offerings/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/httpclient"
)

// Origin is the only host this adapter contacts.
const Origin = "https://codeberg.org"

// Path is the raw offers file of the public repository.
const Path = "/simplicer/ataecina/raw/branch/main/data/offers.csv"

// Header is the exact first line of offers.csv.
var Header = []string{"id", "provider", "plan", "title", "price", "currency", "period", "url", "sponsored", "ends"}

const maxOffers = 100

// Source fetches the published offers.
type Source struct {
	endpoint httpclient.Endpoint
}

// New reads through endpoint.
func New(endpoint httpclient.Endpoint) Source {
	return Source{endpoint: endpoint.Larger(256<<10, 15*time.Second)}
}

// Fetch reads offers.csv once.
func (s Source) Fetch(ctx context.Context) ([]domain.Offer, error) {
	response, err := s.endpoint.Get(ctx, Path, http.Header{"Accept": {"text/csv, text/plain"}})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.endpoint.Host(), err)
	}
	switch response.Status {
	case http.StatusOK:
	case http.StatusNotFound:
		return []domain.Offer{}, nil
	default:
		return nil, fmt.Errorf("%s answered %d", s.endpoint.Host(), response.Status)
	}
	offers, err := Decode(response.Body)
	if err != nil {
		return nil, fmt.Errorf("unexpected offers.csv: %w", err)
	}
	return offers, nil
}

// Decode reads offers.csv: the header, then one offer per line. Every
// link must be https; texts are kept as plain text; a sponsored link says
// "yes"; ends is a date (YYYY-MM-DD) or empty.
func Decode(body []byte) ([]domain.Offer, error) {
	reader := csv.NewReader(bytes.NewReader(body))
	reader.FieldsPerRecord = len(Header)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if !slices.Equal(header, Header) {
		return nil, fmt.Errorf("the header must be %s", strings.Join(Header, ","))
	}
	offers := []domain.Offer{}
	for line := 2; ; line++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return offers, nil
		}
		if err != nil {
			return nil, err
		}
		if len(offers) == maxOffers {
			return nil, fmt.Errorf("more than %d offers", maxOffers)
		}
		offer, err := decodeOffer(record)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		offers = append(offers, offer)
	}
}

func decodeOffer(record []string) (domain.Offer, error) {
	field := func(name string) string { return domain.PlainName(record[slices.Index(Header, name)]) }
	offer := domain.Offer{
		ID: field("id"), Provider: field("provider"), Plan: field("plan"), Title: field("title"),
		Currency: strings.ToUpper(field("currency")), Period: field("period"), Sponsored: field("sponsored") == "yes",
	}
	if offer.ID == "" || offer.Provider == "" || offer.Plan == "" {
		return domain.Offer{}, errors.New("id, provider and plan are required")
	}
	if text := field("price"); text != "" {
		price, err := strconv.ParseFloat(text, 64)
		if err != nil || price < 0 || price > 1e6 {
			return domain.Offer{}, errors.New("price must be a number")
		}
		offer.Price = price
	}
	link, err := url.Parse(strings.TrimSpace(record[slices.Index(Header, "url")]))
	if err != nil || link.Scheme != "https" || link.Host == "" || link.User != nil {
		return domain.Offer{}, errors.New("url must be an https link")
	}
	offer.URL = link.String()
	if text := field("ends"); text != "" {
		ends, err := time.Parse(time.DateOnly, text)
		if err != nil {
			return domain.Offer{}, errors.New("ends must be a date (YYYY-MM-DD)")
		}
		offer.Ends = ends
	}
	return offer, nil
}
