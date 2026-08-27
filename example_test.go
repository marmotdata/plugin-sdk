package pluginsdk

import "fmt"

func ExampleSetColumns() {
	var asset Asset
	_ = SetColumns(&asset, []Column{
		{Name: "id", DataType: "bigint", Nullable: false, PrimaryKey: true},
		{Name: "email", DataType: "text", Nullable: true},
	})

	fmt.Println(asset.Schema["columns"])
	// Output:
	// [{"column_name":"id","data_type":"bigint","is_nullable":false,"is_primary_key":true},{"column_name":"email","data_type":"text","is_nullable":true}]
}

// Embed Column to add source-specific fields.
func ExampleSetColumns_extend() {
	type clickhouseColumn struct {
		Column
		Codec string `json:"codec,omitempty"`
	}

	var asset Asset
	_ = SetColumns(&asset, []clickhouseColumn{
		{Column: Column{Name: "id", DataType: "UInt64"}, Codec: "ZSTD"},
	})

	fmt.Println(asset.Schema["columns"])
	// Output:
	// [{"column_name":"id","data_type":"UInt64","is_nullable":false,"codec":"ZSTD"}]
}

// Pass a []any to mix a plain Column with an extended one in a single call.
func ExampleSetColumns_mix() {
	type clickhouseColumn struct {
		Column
		Codec string `json:"codec,omitempty"`
	}

	var asset Asset
	_ = SetColumns(&asset, []any{
		Column{Name: "id", DataType: "UInt64", PrimaryKey: true},
		clickhouseColumn{Column: Column{Name: "created_at", DataType: "DateTime"}, Codec: "ZSTD"},
	})

	fmt.Println(asset.Schema["columns"])
	// Output:
	// [{"column_name":"id","data_type":"UInt64","is_nullable":false,"is_primary_key":true},{"column_name":"created_at","data_type":"DateTime","is_nullable":false,"codec":"ZSTD"}]
}
