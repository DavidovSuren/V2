// Package content — редактируемый из админки контент (вопросы анкеты и банки
// заданий) с кэшем в памяти. Источник правды — БД; при старте туда
// досеиваются значения по умолчанию из кода и data/*.json.
package content

import (
	"fmt"
	"sync"

	"version20/internal/models"
	"version20/internal/store"
)

type Cache struct {
	st *store.Store

	mu        sync.RWMutex
	questions []models.Question
	banks     map[string][]models.Task
}

// Load досеивает значения по умолчанию (только отсутствующие id) и читает
// актуальный контент из БД.
func Load(st *store.Store, questions []models.Question, male, female []models.Task) (*Cache, error) {
	if err := st.SeedQuestions(questions); err != nil {
		return nil, fmt.Errorf("seed questions: %w", err)
	}
	if err := st.SeedTasks("male", male); err != nil {
		return nil, fmt.Errorf("seed male tasks: %w", err)
	}
	if err := st.SeedTasks("female", female); err != nil {
		return nil, fmt.Errorf("seed female tasks: %w", err)
	}
	c := &Cache{st: st}
	if err := c.Reload(); err != nil {
		return nil, err
	}
	return c, nil
}

// Reload перечитывает контент из БД — вызывается после правок в админке.
func (c *Cache) Reload() error {
	qs, err := c.st.ListQuestions()
	if err != nil {
		return err
	}
	banks := map[string][]models.Task{}
	for _, b := range []string{"male", "female"} {
		if banks[b], err = c.st.ListTasks(b); err != nil {
			return err
		}
	}
	c.mu.Lock()
	c.questions, c.banks = qs, banks
	c.mu.Unlock()
	return nil
}

// Questions и TaskBank возвращают срезы, которые не меняются после Reload
// (Reload подменяет их целиком), поэтому читать их без блокировки безопасно.
func (c *Cache) Questions() []models.Question {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.questions
}

func (c *Cache) TaskBank(gender string) []models.Task {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.banks[gender]
}
