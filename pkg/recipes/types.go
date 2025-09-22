package recipes

// RecipeDefinition defines the structure of a single recipe.
type RecipeDefinition struct {
	Description string            `json:"description"`
	Jobs        map[string]string `json:"jobs"`
}

// RecipeCollection is a map of recipe names to their definitions.
type RecipeCollection map[string]RecipeDefinition