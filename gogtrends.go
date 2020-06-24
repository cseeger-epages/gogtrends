package gogtrends

import (
	"context"
	"net/url"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

var client = newGClient()

// Debug allows to see request-response details.
func Debug(debug bool) {
	client.debug = debug
}

// TrendsCategories return list of available categories for Realtime method as [param]description map.
func TrendsCategories() map[string]string {
	return client.trendsCats
}

// Daily gets daily trends descending ordered by days and articles corresponding to it.
func Daily(ctx context.Context, hl, loc string) ([]*TrendingSearch, error) {
	data, err := client.trends(ctx, gAPI+gDaily, hl, loc)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(data, ")]}',", "", 1)

	out := new(dailyOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	// split searches by days together
	searches := make([]*TrendingSearch, 0)
	for _, v := range out.Default.Searches {
		searches = append(searches, v.Searches...)
	}

	return searches, nil
}

// Realtime represents realtime trends with included articles and sources.
func Realtime(ctx context.Context, hl, loc, cat string) ([]*TrendingStory, error) {
	if !client.validateCategory(cat) {
		return nil, ErrInvalidCategory
	}

	data, err := client.trends(ctx, gAPI+gRealtime, hl, loc, map[string]string{paramCat: cat})
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(data, ")]}'", "", 1)

	out := new(realtimeOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	return out.StorySummaries.TrendingStories, nil
}

// ExploreCategories gets available categories for explore and comparison and caches it in client.
func ExploreCategories(ctx context.Context) (*ExploreCatTree, error) {
	if cats := client.getCategories(); cats != nil {
		return cats, nil
	}

	u, _ := url.Parse(gAPI + gSCategories)

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}'", "", 1)

	out := new(ExploreCatTree)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	// cache in client
	client.setCategories(out)

	return out, nil
}

// ExploreLocations gets available locations for explore and comparison and caches it in client.
func ExploreLocations(ctx context.Context) (*ExploreLocTree, error) {
	if locs := client.getLocations(); locs != nil {
		return locs, nil
	}

	u, _ := url.Parse(gAPI + gSGeo)

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}'", "", 1)

	out := new(ExploreLocTree)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	// cache in client
	client.setLocations(out)

	return out, nil
}

// Explore list of widgets with tokens. Every widget
// is related to specific method (`InterestOverTime`, `InterestOverLoc`, `RelatedSearches`, `Suggestions`)
// and contains required token and request information.
func Explore(ctx context.Context, r *ExploreRequest) (*ExploreData, error) {
	// hook for using incorrect `time` request (backward compatibility)
	for _, r := range r.ComparisonItems {
		r.Time = strings.ReplaceAll(r.Time, "+", " ")
	}

	u, _ := url.Parse(gAPI + gSExplore)

	p := make(url.Values)
	p.Set(paramTZ, "0")

	// marshal request for query param
	mReq, err := jsoniter.MarshalToString(r)
	if err != nil {
		return nil, errors.Wrapf(err, errInvalidRequest)
	}

	p.Set(paramReq, mReq)
	u.RawQuery = p.Encode()

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}'", "", 1)

	out := new(exploreOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	return &ExploreData{out.Widgets[0], out.Widgets[1], out.Widgets[2], out.Widgets[3]}, nil
}

// InterestOverTime as list of `Timeline` dots for chart.
func (e *ExploreData) InterestOverTime(ctx context.Context) ([]*Timeline, error) {
	if e.InterestOverTimeWidget.ID != intOverTimeWidgetID {
		return nil, ErrInvalidWidgetType
	}

	u, _ := url.Parse(gAPI + gSIntOverTime)

	p := make(url.Values)
	p.Set(paramTZ, "0")
	p.Set(paramToken, e.InterestOverTimeWidget.Token)

	for i, v := range e.InterestOverTimeWidget.Request.CompItem {
		if len(v.Geo) == 0 {
			e.InterestOverTimeWidget.Request.CompItem[i].Geo[""] = ""
		}
	}

	// marshal request for query param
	mReq, err := jsoniter.MarshalToString(e.InterestOverTimeWidget.Request)
	if err != nil {
		return nil, errors.Wrapf(err, errInvalidRequest)
	}

	p.Set(paramReq, mReq)
	u.RawQuery = p.Encode()

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}',", "", 1)

	out := new(multilineOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	return out.Default.TimelineData, nil
}

// InterestByLocation as list of `GeoMap`, with geo codes and interest values.
func (e ExploreData) InterestByLocation(ctx context.Context) ([]*GeoMap, error) {
	if e.InterestByLocationWidget.ID != intOverRegionID {
		return nil, ErrInvalidWidgetType
	}

	u, _ := url.Parse(gAPI + gSIntOverReg)

	p := make(url.Values)
	p.Set(paramTZ, "0")
	p.Set(paramToken, e.InterestByLocationWidget.Token)

	if len(e.InterestByLocationWidget.Request.CompItem) > 1 {
		e.InterestByLocationWidget.Request.DataMode = compareDataMode
	}

	// marshal request for query param
	mReq, err := jsoniter.MarshalToString(e.InterestByLocationWidget.Request)
	if err != nil {
		return nil, errors.Wrapf(err, errInvalidRequest)
	}

	p.Set(paramReq, mReq)
	u.RawQuery = p.Encode()

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}',", "", 1)

	out := new(geoOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	return out.Default.GeoMapData, nil
}

// RelatedTopics .
func (e ExploreData) RelatedTopics(ctx context.Context) ([]*RankedKeyword, error) {
	return related(ctx, e.RelatedTopicsWidget)
}

// RelatedQuerys .
func (e ExploreData) RelatedQuerys(ctx context.Context) ([]*RankedKeyword, error) {
	return related(ctx, e.RelatedQuerysWidget)
}

// related topics or queries, list of `RankedKeyword`, supports two types of widgets.
func related(ctx context.Context, w *ExploreWidget) ([]*RankedKeyword, error) {
	if w.ID != relatedQueriesID && w.ID != relatedTopicsID {
		return nil, ErrInvalidWidgetType
	}

	u, _ := url.Parse(gAPI + gSRelated)

	p := make(url.Values)
	p.Set(paramTZ, "0")
	p.Set(paramToken, w.Token)

	if len(w.Request.Restriction.Geo) == 0 {
		w.Request.Restriction.Geo[""] = ""
	}

	// marshal request for query param
	mReq, err := jsoniter.MarshalToString(w.Request)
	if err != nil {
		return nil, errors.Wrapf(err, errInvalidRequest)
	}

	p.Set(paramReq, mReq)
	u.RawQuery = p.Encode()

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}',", "", 1)

	out := new(relatedOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	// split all keywords together
	keywords := make([]*RankedKeyword, 0)
	for _, v := range out.Default.Ranked {
		keywords = append(keywords, v.Keywords...)
	}

	return keywords, nil
}
