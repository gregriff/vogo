package routes

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/gregriff/vogo/server/internal/crypto"
	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/schemas"
	"github.com/gregriff/vogo/server/internal/validation"
)

func (h *RouteHandler) Register(w http.ResponseWriter, req *http.Request) {
	rData := schemas.NewUserRequest{}
	if err := json.NewDecoder(req.Body).Decode(&rData); err != nil {
		panic(err)
	}
	log.Printf("new user parsed: %#v", rData)

	vErr, statusCode := validation.CheckRegistrationCredentials(h.db, rData.InviteCode, rData.Username, rData.Password)
	if vErr != nil {
		http.Error(w, vErr.Error(), statusCode)
		return
	}

	hashedPassword, hashErr := crypto.HashPassword(rData.Password)
	if hashErr != nil {
		log.Println(hashErr.Error())
		err := errors.New("password error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	username, sqlErr := dal.CreateUser(h.db, rData.Username, hashedPassword, rData.InviteCode)
	if sqlErr != nil {
		log.Println(sqlErr.Error())
		err := errors.New("error creating new user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJSON(w, &username)
}
