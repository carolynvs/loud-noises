package main

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
)

const (
	SessionName   = "slackoverload-auth"
	SessionUserId = "user-id"
)

type SessionStore struct {
	store sessions.Store
}

func (s *SessionStore) Init() error {
	sessionKey, err := s.getSessionKey()
	if err != nil {
		return err
	}

	s.store = sessions.NewCookieStore([]byte(sessionKey))
	return nil
}

func (s *SessionStore) getSessionKey() (string, error) {
	sessionKey, _, err := getSecret("session-key")
	return sessionKey, err
}

func (s *SessionStore) GetCurrentSession(request *http.Request, writer http.ResponseWriter) (Session, error) {
	session, err := s.store.Get(request, SessionName)
	if err != nil {
		return Session{}, errors.Wrap(err, "error loading session for current user")
	}
	return Session{session: session, request: request, writer: writer}, nil
}

type Session struct {
	session *sessions.Session
	request *http.Request
	writer  http.ResponseWriter
}

func (s Session) GetUserId() string {
	userId, ok := s.session.Values[SessionUserId]
	if !ok {
		return ""
	}
	return userId.(string)
}

func (s Session) SetUserId(value string) {
	s.session.Values[SessionUserId] = value
}

func (s Session) Save() error {
	err := s.session.Save(s.request, s.writer)
	return errors.Wrapf(err, "error saving session for user %s", s.GetUserId())
}
