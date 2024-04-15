package mods

// Category is used to describe a mod.
// Mods can only belong to a single category.
type Category string

const (
	NoCategory    Category = "no-category"
	Content                = "content"       // Mods introducing new content into the game.
	Overhaul               = "overhaul"      // Large total conversion mods.
	Tweaks                 = "tweaks"        // Small changes concerning balance, gameplay, or graphics.
	Utilities              = "utilities"     // Providing the player with new tools or adjusting the game interface, without fundamentally changing gameplay.
	Scenarios              = "scenarios"     // Scenarios, maps, puzzles.
	ModPacks               = "mod-packs"     // Collections of mods with tweaks to make them work together.
	Localizations          = "localizations" // Translations for other mods.
	Internal               = "internal"      // Lua libraries for use by other mods and submods that are parts of a larger mod.
)

// Categories returns a list of all available categories.
func Categories() []string {
	return []string{
		"",
		string(NoCategory),
		string(Content),
		string(Overhaul),
		string(Tweaks),
		string(Utilities),
		string(Scenarios),
		string(ModPacks),
		string(Localizations),
		string(Internal),
	}
}
