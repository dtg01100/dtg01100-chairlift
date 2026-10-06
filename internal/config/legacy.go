package config

import "gopkg.in/yaml.v3"

// system_page is a compatibility input, not a navigable page. Validate its
// historical inventory before migration, including values later superseded
// or retired, so compatibility cannot conceal typos or bypass sudo checks.
func validateLegacySystemPage(src configSource, value *yaml.Node) *LoadError {
	if value.Kind == yaml.ScalarNode && value.Tag == "!!null" {
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return validatorPageValueShapeError(src.path, "system_page", value)
	}
	return validateNamedGroupEntries(src, []string{
		"system_info_group", "bootc_status_group", "channel_group", "health_group",
	}, value)
}

// legacyMaintenanceGroups names the maintenance_page entries carried by
// pre-26.09 Bluefin releases (v0.12.x) and removed by 4141e8c when
// maintenance was consolidated into maintenance_cleanup_group /
// maintenance_freespace_group / reset_group. Installed hosts still ship
// /usr/share/chairlift/config.yml with these groups in place; accepting
// the names without ever activating the underlying behavior keeps those
// hosts runnable while a single source of truth for the cleanup actions
// lives in the current schema.
var legacyMaintenanceGroups = []string{
	"maintenance_brew_group",
	"maintenance_flatpak_group",
	"maintenance_optimization_group",
}

// legacyUpdatesGroups names the updates_page entries carried by
// pre-26.09 Bluefin releases (v0.12.x) and removed by 4141e8c when the
// Update All coordinator absorbed update_all_group, sysupdate_updates_group
// was folded into the per-provider groups, and automatic_updates_group
// took its place as the schedule switch. Pre-26.09 host files that have
// not been refreshed still ship /usr/share/chairlift/config.yml with
// these names; accepting them keeps those hosts runnable without
// re-activating the old behavior.
var legacyUpdatesGroups = []string{
	"update_all_group",
	"sysupdate_updates_group",
}

// legacyFeaturesGroups names the features_page entries carried by
// pre-26.09 Bluefin releases (v0.12.x) and removed by 4141e8c when the
// Local AI destination moved to its own Agents page. Pre-26.09 host
// files that have not been refreshed still ship
// /usr/share/chairlift/config.yml with this group in place; accepting it
// keeps those hosts runnable without re-activating the old features
// layout.
var legacyFeaturesGroups = []string{
	"ai_group",
}

// legacyGroupNames returns the legacy group names accepted alongside the
// canonical schema for the given page. Returns nil when no page-specific
// compatibility names exist for the page, which is the common case.
func legacyGroupNames(page string) []string {
	switch page {
	case "maintenance_page":
		return legacyMaintenanceGroups
	case "updates_page":
		return legacyUpdatesGroups
	case "features_page":
		return legacyFeaturesGroups
	default:
		return nil
	}
}

// stripLegacyMaintenanceGroups removes retired maintenance_page,
// updates_page, and features_page groups from the AST prior to decoding
// so they never reach runtime Config. Validation has already accepted
// them as known, so the strip cannot hide a shape or value error; an
// undeclared field under a retired group name still fails closed before
// this runs.
//
// Renamed from the original maintenance-only form to reflect the wider
// scope; the previous single-page stripper is preserved as
// stripLegacyGroupFromPage for reuse from the generic strip pass.
func stripLegacyMaintenanceGroups(top *yaml.Node) {
	for _, page := range []string{"maintenance_page", "updates_page", "features_page"} {
		stripLegacyGroupFromPage(top, page, legacyGroupNames(page))
	}
}

// stripLegacyGroupFromPage removes every retired group listed under a
// page's mapping value. The page node is left in place (with its other
// canonical groups intact); only the legacy names vanish. A nil page
// node or one whose value is not a mapping is a no-op.
func stripLegacyGroupFromPage(top *yaml.Node, page string, retired []string) {
	if len(retired) == 0 {
		return
	}
	pageNode := mappingValue(top, page)
	if pageNode == nil || pageNode.Kind != yaml.MappingNode {
		return
	}
	retiredSet := make(map[string]bool, len(retired))
	for _, g := range retired {
		retiredSet[g] = true
	}
	newContent := make([]*yaml.Node, 0, len(pageNode.Content))
	for i := 0; i+1 < len(pageNode.Content); i += 2 {
		key := pageNode.Content[i]
		val := pageNode.Content[i+1]
		if retiredSet[key.Value] {
			continue
		}
		newContent = append(newContent, key, val)
	}
	pageNode.Content = newContent
}

// migrateLegacySystemPage runs only after source-graph and schema validation.
// The effective tree is alias-free and privately owned. Move surviving groups
// to Updates, preserving explicit false values. Current non-null fields win;
// nulls remain no-op overlays, just as they are in the ordinary config merge.
// No source file is rewritten and retired groups never reach runtime Config.
func migrateLegacySystemPage(top *yaml.Node) {
	legacy := mappingValue(top, "system_page")
	if legacy == nil || legacy.Kind != yaml.MappingNode {
		return
	}
	updates := mappingValue(top, "updates_page")
	for _, name := range []string{"bootc_status_group", "channel_group"} {
		group := mappingValue(legacy, name)
		if group == nil || group.Kind != yaml.MappingNode {
			continue
		}
		if updates == nil {
			updates = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			top.Content = append(top.Content, stringKey("updates_page"), updates)
		} else if updates.Kind != yaml.MappingNode {
			*updates = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		current := mappingValue(updates, name)
		if current == nil {
			updates.Content = append(updates.Content, stringKey(name), group)
		} else if current.Kind != yaml.MappingNode {
			*current = *group
		} else {
			for i := 0; i < len(group.Content); i += 2 {
				key, value := group.Content[i], group.Content[i+1]
				field := mappingValue(current, key.Value)
				if field == nil {
					current.Content = append(current.Content, key, value)
				} else if field.Tag == "!!null" {
					*field = *value
				}
			}
		}
	}
}

func mappingValue(node *yaml.Node, name string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == name {
			return node.Content[i+1]
		}
	}
	return nil
}

func stringKey(name string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}
}
