package handlers

import (
	"log"
	"net/http"
)

func (a *App) serverError(w http.ResponseWriter, err error) {
	log.Println("[error]", err)
	http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
}
