package models

// Task — запись банка заданий (data/tasks_male.json, tasks_female.json),
// сгенерированного backend/scripts/generate-tasks.js в Node-версии.
type Task struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Gender   string `json:"gender"` // "male" | "female" | "both"
	Text     string `json:"text"`
	Why      string `json:"why"`
}
