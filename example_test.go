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

// Embed Column to carry source-specific fields alongside the canonical ones,
// then build the list with append and set it once.
func ExampleSetColumns_extend() {
	type clickhouseColumn struct {
		Column
		Codec string `json:"codec,omitempty"`
	}

	var cols []clickhouseColumn
	cols = append(cols, clickhouseColumn{Column: Column{Name: "id", DataType: "UInt64", PrimaryKey: true}})
	cols = append(cols, clickhouseColumn{
		Column: Column{Name: "created_at", DataType: "DateTime"},
		Codec:  "Delta, ZSTD",
	})

	var asset Asset
	_ = SetColumns(&asset, cols)

	fmt.Println(asset.Schema["columns"])
	// Output:
	// [{"column_name":"id","data_type":"UInt64","is_nullable":false,"is_primary_key":true},{"column_name":"created_at","data_type":"DateTime","is_nullable":false,"codec":"Delta, ZSTD"}]
}
