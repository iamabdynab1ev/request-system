package services

var OrderValidationRules = map[string][]string{
	"EQUIPMENT": {
		"equipment_id",
		"equipment_type_id",
		"priority_id",
		"status_id",
		

	},

	"ADMINISTRATIVE": {
		"priority_id",
		"status_id",
		
	},
}
