package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	int64plan "github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func int64planUseStateForUnknown() planmodifier.Int64 {
	return int64plan.UseStateForUnknown()
}

func stringplanUseStateForUnknown() planmodifier.String {
	return stringplanmodifier.UseStateForUnknown()
}

func setToIntSlice(ctx context.Context, set types.Set) ([]int, diag.Diagnostics) {
	var diags diag.Diagnostics
	if set.IsNull() || set.IsUnknown() {
		return nil, diags
	}
	var ids []int64
	diags.Append(set.ElementsAs(ctx, &ids, false)...)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]int, len(ids))
	for i, v := range ids {
		out[i] = int(v)
	}
	return out, diags
}

func intSliceToSet(ctx context.Context, in []int) (types.Set, diag.Diagnostics) {
	vals := make([]int64, len(in))
	for i, v := range in {
		vals[i] = int64(v)
	}
	return types.SetValueFrom(ctx, types.Int64Type, vals)
}

func setToStringSlice(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if set.IsNull() || set.IsUnknown() {
		return nil, diags
	}
	var vals []string
	diags.Append(set.ElementsAs(ctx, &vals, false)...)
	return vals, diags
}

func listToStringSlice(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}
	var vals []string
	diags.Append(list.ElementsAs(ctx, &vals, false)...)
	return vals, diags
}

func stringSliceToList(ctx context.Context, in []string) (types.List, diag.Diagnostics) {
	if in == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, in)
}

// diffIntSlices returns (added, removed) — elements in desired but not current, and vice versa.
func diffIntSlices(desired, current []int) (added, removed []int) {
	d := make(map[int]bool, len(desired))
	for _, v := range desired {
		d[v] = true
	}
	c := make(map[int]bool, len(current))
	for _, v := range current {
		c[v] = true
	}
	for v := range d {
		if !c[v] {
			added = append(added, v)
		}
	}
	for v := range c {
		if !d[v] {
			removed = append(removed, v)
		}
	}
	return
}

// stringOrNil returns *string for an empty-aware Optional Terraform value.
// Returns nil when null or unknown, otherwise a pointer to the value.
func stringPtrFromTF(s types.String) *string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}
	v := s.ValueString()
	return &v
}

func stringFromPtr(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

func boolPtrFromTF(b types.Bool) *bool {
	if b.IsNull() || b.IsUnknown() {
		return nil
	}
	v := b.ValueBool()
	return &v
}

func boolFromPtr(p *bool) types.Bool {
	if p == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*p)
}
