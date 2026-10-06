package config

import (
	"reflect"

	"gopkg.in/yaml.v3"
)

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
	return validateNamedGroupEntries(src, "system_page", []string{
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
// pre-26.09 Bluefin releases (v0.12.x) and moved or removed by 4141e8c:
// ai_group was retired when the Local AI destination moved to its own
// Agents page, and troubleshooting_group moved to help_page. Pre-26.09 host
// files that have not been refreshed still ship
// /usr/share/chairlift/config.yml with these groups in place; accepting
// them keeps those hosts runnable without re-activating the old features
// layout. troubleshooting_group is migrated to help_page (see
// migrateLegacyFeaturesPage) before the features_page copy is stripped.
var legacyFeaturesGroups = []string{
	"ai_group",
	"troubleshooting_group",
}

// legacyGroupFieldTypes returns the retired fields a legacy group accepted
// in pre-26.09 releases, keyed by YAML name with their historical Go types,
// so validation can type-check them. Only ai_group carried such fields
// (ai_images, ai_model); the group is stripped before decoding, so they
// never reach runtime Config.
func legacyGroupFieldTypes(page, group string) map[string]reflect.Type {
	if page == "features_page" && group == "ai_group" {
		return map[string]reflect.Type{
			"ai_images": reflect.TypeOf(map[string]string{}),
			"ai_model":  reflect.TypeOf(""),
		}
	}
	return nil
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

// stripLegacyGroups removes retired maintenance_page, updates_page, and
// features_page groups from the AST prior to decoding so they never reach
// runtime Config. Validation has already accepted them as known, so the
// strip cannot hide a shape or value error; an undeclared field under a
// retired group name still fails closed before this runs.
func stripLegacyGroups(top *yaml.Node) {
	for _, page := range []string{"maintenance_page", "updates_page", "features_page"} {
		stripLegacyGroupFromPage(top, page, legacyGroupNames(page))
	}
}

// migrateLegacyFeaturesPage moves a pre-26.09 features_page
// troubleshooting_group to help_page, where 4141e8c relocated it, with the
// same precedence as the system_page migration: current non-null help_page
// fields win. This preserves an administrator's explicit opt-out. It runs
// after validation and before stripLegacyGroups removes the old copy.
func migrateLegacyFeaturesPage(top *yaml.Node) {
	legacy := mappingValue(top, "features_page")
	if legacy == nil || legacy.Kind != yaml.MappingNode {
		return
	}
	migrateLegacyGroup(top, legacy, "help_page", "troubleshooting_group")
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
	for _, name := range []string{"bootc_status_group", "channel_group"} {
		migrateLegacyGroup(top, legacy, "updates_page", name)
	}
}

// migrateLegacyGroup overlays legacy[name] onto top[toPage][name], creating
// the page or group when absent. Current non-null fields win; nulls are
// no-op overlays.
func migrateLegacyGroup(top, legacy *yaml.Node, toPage, name string) {
	group := mappingValue(legacy, name)
	if group == nil || group.Kind != yaml.MappingNode {
		return
	}
	target := mappingValue(top, toPage)
	if target == nil {
		target = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		top.Content = append(top.Content, stringKey(toPage), target)
	} else if target.Kind != yaml.MappingNode {
		*target = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	current := mappingValue(target, name)
	if current == nil {
		target.Content = append(target.Content, stringKey(name), group)
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
