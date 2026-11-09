package mrn

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLowercasesEveryPart(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql/shop.public.orders",
		New("Table", "PostgreSQL", "Shop.Public.Orders"))
}

func TestNewReplacesSpaceInService(t *testing.T) {
	// Providers are display strings, so "Unity Catalog" is what the UI
	// shows and what the MRN has to survive without a space.
	assert.Equal(t, "mrn://asset/table/unity-catalog/shop.sales.orders",
		New("Table", "Unity Catalog", "shop.sales.orders"))
}

func TestNewReplacesSpaceInType(t *testing.T) {
	assert.Equal(t, "mrn://asset/data-stream/elasticsearch/logs",
		New("Data Stream", "Elasticsearch", "logs"))
}

func TestNewReplacesSpaceInName(t *testing.T) {
	assert.Equal(t, "mrn://asset/dashboard/grafana/quarterly-revenue",
		New("Dashboard", "Grafana", "Quarterly Revenue"))
}

func TestNewReplacesTabsAndNewlines(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/hive/a-b-c", New("Table", "Hive", "a\tb\nc"))
}

func TestNewReplacesSlashes(t *testing.T) {
	// A slash would add a component that Parse cannot tell apart from
	// the type, service and name.
	assert.Equal(t, "mrn://asset/deployment/kubernetes/payments-api",
		New("Deployment", "Kubernetes", "payments/api"))
}

func TestNewNeverEmitsWhitespace(t *testing.T) {
	built := New("Data Stream", "Unity Catalog", "quarterly revenue\treport")
	assert.NotContains(t, built, " ")
	assert.NotContains(t, built, "\t")
}

func TestAssetBuildsKindedMRN(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql/analytics.public.orders",
		Asset("Table", "PostgreSQL", "Analytics", "public", "orders").String())
}

func TestAssetWithZone(t *testing.T) {
	m := Asset("Table", "PostgreSQL", "analytics", "public", "orders").WithZone("Prod EU")
	assert.Equal(t, "mrn://asset/table/postgresql@prod-eu/analytics.public.orders", m.String())
}

func TestZoneCannotSmuggleAnotherAt(t *testing.T) {
	m := Asset("Table", "PostgreSQL", "orders").WithZone("a@b")
	assert.Equal(t, "mrn://asset/table/postgresql@a-b/orders", m.String())
}

func TestSinglePartNameKeepsDots(t *testing.T) {
	assert.Equal(t, "mrn://asset/topic/kafka/orders.v1", Asset("Topic", "Kafka", "orders.v1").String())
}

func TestMultiPartNameQuotesDots(t *testing.T) {
	m := Asset("Table", "DuckDB", "my.file", "main", "orders")
	assert.Equal(t, `mrn://asset/table/duckdb/"my.file".main.orders`, m.String())
	assert.Equal(t, []string{"my.file", "main", "orders"}, m.Path())
}

func TestQuotesInsidePartsAreDoubled(t *testing.T) {
	m := Asset("Table", "Postgres", `we"ird.db`, "t")
	assert.Equal(t, `"we""ird.db".t`, m.Name)
	assert.Equal(t, []string{`we"ird.db`, "t"}, m.Path())
}

func TestParseLegacyIsAnAsset(t *testing.T) {
	m, err := ParseMRN("mrn://table/postgresql/orders")
	require.NoError(t, err)
	assert.True(t, m.Legacy)
	assert.Equal(t, KindAsset, m.Kind)
	assert.Equal(t, "mrn://asset/table/postgresql/orders", m.String())
	assert.Equal(t, "mrn://table/postgresql/orders", m.LegacyString())
}

func TestParseLegacyCanonicalisesCaseAndSpaces(t *testing.T) {
	c, err := Canonical("MRN://Table/Unity Catalog/Shop.Orders")
	require.NoError(t, err)
	assert.Equal(t, "mrn://asset/table/unity-catalog/shop.orders", c)
}

func TestParseKindedAssetWithZone(t *testing.T) {
	m, err := ParseMRN("mrn://asset/table/postgresql@prod-eu/analytics.public.orders")
	require.NoError(t, err)
	assert.False(t, m.Legacy)
	assert.Equal(t, "postgresql", m.Provider)
	assert.Equal(t, "prod-eu", m.Zone)
	assert.Equal(t, "postgresql@prod-eu", m.ProviderSegment())
	assert.Equal(t, []string{"analytics", "public", "orders"}, m.Path())
}

func TestCanonicalIsIdempotent(t *testing.T) {
	for _, s := range []string{
		"mrn://table/postgresql/orders",
		"mrn://asset/table/postgresql@prod/a.b.c",
		`mrn://asset/table/duckdb@alice-laptop/"my.file".main.orders`,
		"mrn://glossary-term/marmot/finance.revenue.arr",
		"mrn://data-product/customer-360",
	} {
		once, err := Canonical(s)
		require.NoError(t, err, s)
		twice, err := Canonical(once)
		require.NoError(t, err)
		assert.Equal(t, once, twice)
	}
}

func TestUnicodeNormalisesToNFC(t *testing.T) {
	composed := Asset("Table", "Postgres", "café").String()
	decomposed := Asset("Table", "Postgres", "café").String()
	assert.Equal(t, composed, decomposed)
}

func TestOtherKinds(t *testing.T) {
	assert.Equal(t, "mrn://glossary-term/marmot/finance.revenue.arr", GlossaryTerm("marmot", "Finance", "Revenue", "ARR").String())
	assert.Equal(t, "mrn://data-product/customer-360", DataProduct("Customer 360").String())
	assert.Equal(t, "mrn://team/data-platform", Team("Data  Platform!").String())
	assert.Equal(t, "mrn://pipeline/shop-pg", Pipeline("shop-pg").String())
	assert.Equal(t, "mrn://source/warehouse", Source("Warehouse").String())

	m, err := ParseMRN("mrn://data-product/customer-360")
	require.NoError(t, err)
	assert.Equal(t, KindDataProduct, m.Kind)
}

func TestReservedKindWinsOverLegacyType(t *testing.T) {
	m, err := ParseMRN("mrn://glossary-term/marmot/arr")
	require.NoError(t, err)
	assert.Equal(t, KindGlossaryTerm, m.Kind)
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "orders", "postgres://db/schema/table", "mrn://a/b/c/d/e", "mrn://table//orders"} {
		_, err := ParseMRN(s)
		assert.ErrorIs(t, err, ErrInvalid, s)
	}
}

func TestHostHint(t *testing.T) {
	m := Asset("Table", "PostgreSQL", "shop", "public", "orders").WithHost("PG-EU.internal:5432")
	assert.Equal(t, "mrn://asset/table/postgresql@host=pg-eu.internal:5432/shop.public.orders", m.String())
	parsed, err := ParseMRN(m.String())
	require.NoError(t, err)
	host, ok := parsed.HostHint()
	assert.True(t, ok)
	assert.Equal(t, "pg-eu.internal:5432", host)
}

func TestTextMarshalling(t *testing.T) {
	var m MRN
	require.NoError(t, m.UnmarshalText([]byte("mrn://topic/kafka/orders")))
	b, err := m.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "mrn://asset/topic/kafka/orders", string(b))
}

func TestThreeSegmentsStartingWithAssetIsLegacy(t *testing.T) {
	// A kinded asset always has four segments, so three can only be a legacy
	// MRN whose type happens to be "asset".
	m, err := ParseMRN("mrn://asset/acme/widget")
	require.NoError(t, err)
	assert.True(t, m.Legacy)
	assert.Equal(t, "asset", m.Type)
	assert.Equal(t, "mrn://asset/asset/acme/widget", m.String())
}

func TestNewRoundTripsThroughParse(t *testing.T) {
	original := New("Data Stream", "Unity Catalog", "shop.sales.orders")
	parsed, err := ParseMRN(original)
	require.NoError(t, err)
	assert.Equal(t, original, parsed.String())
	assert.Equal(t, "data-stream", parsed.Type)
	assert.Equal(t, "unity-catalog", parsed.Provider)
}
