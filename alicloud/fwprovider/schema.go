package fwprovider

import (
	"fmt"

	fwschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// convertProviderSchema builds the framework provider schema from the SDKv2
// provider schema. The muxed provider requires both servers to return an
// identical provider schema (tf5muxserver compares the full tfprotov5.Schema,
// ignoring only attribute/block ordering and Min/MaxItems), so deriving the
// framework schema directly from the SDKv2 one guarantees parity by
// construction rather than by hand-transcription.
func convertProviderSchema(in map[string]*sdkschema.Schema) fwschema.Schema {
	attributes, blocks := convertSchemaMap(in)
	return fwschema.Schema{
		Attributes: attributes,
		Blocks:     blocks,
	}
}

// convertSchemaMap splits an SDKv2 schema map into framework attributes and
// nested blocks, converting each entry.
func convertSchemaMap(in map[string]*sdkschema.Schema) (map[string]fwschema.Attribute, map[string]fwschema.Block) {
	attributes := make(map[string]fwschema.Attribute)
	blocks := make(map[string]fwschema.Block)

	for name, s := range in {
		if isNestedBlock(s) {
			blocks[name] = convertBlock(name, s)
		} else {
			attributes[name] = convertAttribute(name, s)
		}
	}

	return attributes, blocks
}

// isNestedBlock reports whether an SDKv2 schema entry is emitted as a nested
// block (a TypeList/TypeSet whose Elem is a *schema.Resource) rather than a
// scalar attribute.
func isNestedBlock(s *sdkschema.Schema) bool {
	if s.Type != sdkschema.TypeList && s.Type != sdkschema.TypeSet {
		return false
	}
	_, ok := s.Elem.(*sdkschema.Resource)
	return ok
}

// convertAttribute converts a scalar SDKv2 schema entry into the matching
// framework attribute. Only the scalar types used by the provider schema are
// supported; anything else panics so schema drift surfaces immediately instead
// of producing a silent parity mismatch. Provider-config attributes are never
// Computed, so only Optional/Required are carried over.
func convertAttribute(name string, s *sdkschema.Schema) fwschema.Attribute {
	optional, required := effectiveOptionalRequired(s)

	switch s.Type {
	case sdkschema.TypeString:
		return fwschema.StringAttribute{
			Optional:           optional,
			Required:           required,
			Description:        s.Description,
			DeprecationMessage: s.Deprecated,
		}
	case sdkschema.TypeBool:
		return fwschema.BoolAttribute{
			Optional:           optional,
			Required:           required,
			Description:        s.Description,
			DeprecationMessage: s.Deprecated,
		}
	case sdkschema.TypeInt:
		return fwschema.Int64Attribute{
			Optional:           optional,
			Required:           required,
			Description:        s.Description,
			DeprecationMessage: s.Deprecated,
		}
	default:
		panic(fmt.Sprintf("fwprovider: unsupported provider schema attribute type %q for %q; extend convertAttribute to keep mux schema parity", s.Type.String(), name))
	}
}

// effectiveOptionalRequired mirrors terraform-plugin-sdk/v2's
// coreConfigSchemaAttribute: a Required field with a DefaultFunc is emitted at
// the protocol level as Optional when the DefaultFunc yields a value (or an
// error), because the default can satisfy the requirement. Replicating this
// exactly is required for mux provider-schema parity. This runs in the same
// process/environment as the SDKv2 provider, so DefaultFunc evaluates
// identically on both sides.
func effectiveOptionalRequired(s *sdkschema.Schema) (optional bool, required bool) {
	required = s.Required
	optional = s.Optional
	if required && s.DefaultFunc != nil {
		v, err := s.DefaultFunc()
		if err != nil || v != nil {
			required = false
			optional = true
		}
	}
	return optional, required
}

// convertBlock converts an SDKv2 nested block (TypeList/TypeSet with a
// *schema.Resource Elem) into the matching framework nested block. TypeSet maps
// to SetNestedBlock and TypeList to ListNestedBlock so the protocol nesting mode
// matches. Inner attributes and blocks are converted recursively.
func convertBlock(name string, s *sdkschema.Schema) fwschema.Block {
	resource := s.Elem.(*sdkschema.Resource)
	attributes, blocks := convertSchemaMap(resource.Schema)
	nested := fwschema.NestedBlockObject{
		Attributes: attributes,
		Blocks:     blocks,
	}

	switch s.Type {
	case sdkschema.TypeSet:
		return fwschema.SetNestedBlock{
			NestedObject:       nested,
			Description:        s.Description,
			DeprecationMessage: s.Deprecated,
		}
	case sdkschema.TypeList:
		return fwschema.ListNestedBlock{
			NestedObject:       nested,
			Description:        s.Description,
			DeprecationMessage: s.Deprecated,
		}
	default:
		panic(fmt.Sprintf("fwprovider: unsupported provider schema block type %q for %q", s.Type.String(), name))
	}
}
