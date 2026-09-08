package mrn

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLowercasesEveryPart(t *testing.T) {
	assert.Equal(t, "mrn://table/postgresql/shop.public.orders",
		New("Table", "PostgreSQL", "Shop.Public.Orders"))
}

func TestNewReplacesSpaceInService(t *testing.T) {
	// Providers are display strings, so "Unity Catalog" is what the UI
	// shows and what the MRN has to survive without a space.
	assert.Equal(t, "mrn://table/unity-catalog/shop.sales.orders",
		New("Table", "Unity Catalog", "shop.sales.orders"))
}

func TestNewReplacesSpaceInType(t *testing.T) {
	assert.Equal(t, "mrn://data-stream/elasticsearch/logs",
		New("Data Stream", "Elasticsearch", "logs"))
}

func TestNewReplacesSpaceInName(t *testing.T) {
	assert.Equal(t, "mrn://dashboard/grafana/quarterly-revenue",
		New("Dashboard", "Grafana", "Quarterly Revenue"))
}

func TestNewReplacesTabsAndNewlines(t *testing.T) {
	assert.Equal(t, "mrn://table/hive/a-b-c", New("Table", "Hive", "a\tb\nc"))
}

func TestNewReplacesSlashes(t *testing.T) {
	// A slash would add a component that Parse cannot tell apart from
	// the type, service and name.
	assert.Equal(t, "mrn://deployment/kubernetes/payments-api",
		New("Deployment", "Kubernetes", "payments/api"))
}

func TestNewNeverEmitsWhitespace(t *testing.T) {
	built := New("Data Stream", "Unity Catalog", "quarterly revenue\treport")
	assert.NotContains(t, built, " ")
	assert.NotContains(t, built, "\t")
}

func TestParseSplitsThreeComponents(t *testing.T) {
	parsed, err := Parse("mrn://table/unity-catalog/shop.sales.orders")
	require.NoError(t, err)

	assert.Equal(t, "table", parsed.Type)
	assert.Equal(t, "unity-catalog", parsed.Service)
	assert.Equal(t, "shop.sales.orders", parsed.Name)
}

func TestParseRejectsWrongComponentCount(t *testing.T) {
	_, err := Parse("mrn://table/postgresql")
	require.Error(t, err)
}

func TestRoundTripIsStable(t *testing.T) {
	// The host rebuilds an MRN from the parsed parts, so a built MRN has
	// to survive Parse then New byte-identical.
	original := New("Data Stream", "Unity Catalog", "shop.sales.orders")

	parsed, err := Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, New(parsed.Type, parsed.Service, parsed.Name))
}
