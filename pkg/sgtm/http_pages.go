package sgtm

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/go-chi/chi"
	packr "github.com/gobuffalo/packr/v2"
	"go.uber.org/zap"
	"moul.io/sgtm/pkg/sgtmpb"
)

func (svc *Service) indexPage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplate(box, "index.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		if svc.opts.DevMode {
			tmpl = loadTemplate(box, "index.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.ExecuteTemplate(w, "base", &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) settingsPage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplate(box, "settings.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		if svc.opts.DevMode {
			tmpl = loadTemplate(box, "settings.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.Execute(w, &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) profilePage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplate(box, "profile.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}

		userSlug := chi.URLParam(r, "user_slug")
		if err := svc.db.Where(sgtmpb.User{Slug: userSlug}).First(&data.Profile.User).Error; err != nil {
			data.Error = err.Error()
		}

		if svc.opts.DevMode {
			tmpl = loadTemplate(box, "profile.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.Execute(w, &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) newTemplateData(r *http.Request) (*templateData, error) {
	data := templateData{
		Title: "SGTM",
		Date:  time.Now(),
		Opts:  svc.opts.Filtered(),
		Lang:  "en", // FIXME: dynamic
	}

	if cookie, err := r.Cookie(oauthTokenCookie); err == nil {
		data.JWTToken = cookie.Value
		var err error
		data.Claims, err = svc.parseJWTToken(data.JWTToken)
		if err != nil {
			return nil, fmt.Errorf("parse jwt token: %w", err)
		}
		if err := svc.db.First(&data.User, data.Claims.Session.UserID).Error; err != nil {
			svc.logger.Warn("load user from DB", zap.Error(err))
		}
	}

	return &data, nil
}

func loadTemplate(box *packr.Box, filepath string) *template.Template {
	src, err := box.FindString(filepath)
	if err != nil {
		panic(err)
	}
	base, err := box.FindString("base.tmpl.html")
	if err != nil {
		panic(err)
	}
	allInOne := strings.Join([]string{
		strings.TrimSpace(src),
		strings.TrimSpace(base),
	}, "\n")
	tmpl, err := template.New("index").Funcs(sprig.FuncMap()).Parse(allInOne)
	if err != nil {
		panic(err)
	}
	return tmpl
}

type templateData struct {
	// common

	Title    string
	Date     time.Time
	JWTToken string
	Claims   *jwtClaims
	Duration time.Duration
	Opts     Opts
	Lang     string
	User     sgtmpb.User
	Error    string

	// specific

	Index    struct{}
	Settings struct{}
	Profile  struct{ User sgtmpb.User }
}
