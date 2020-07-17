package sgtm

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	sprig "github.com/Masterminds/sprig/v3"
	humanize "github.com/dustin/go-humanize"
	"github.com/go-chi/chi"
	packr "github.com/gobuffalo/packr/v2"
	striptags "github.com/grokify/html-strip-tags-go"
	"github.com/hako/durafmt"
	"github.com/yanatan16/golang-soundcloud/soundcloud"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"moul.io/godev"
	"moul.io/sgtm/pkg/sgtmpb"
)

func (svc *Service) homePage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplates(box, "base.tmpl.html", "home.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom

		// tracking
		{
			viewEvent := sgtmpb.Post{AuthorID: data.UserID, Kind: sgtmpb.Post_ViewHomeKind}
			if err := svc.db.Create(&viewEvent).Error; err != nil {
				data.Error = "Cannot write activity: " + err.Error()
			} else {
				svc.logger.Debug("new view home", zap.Any("event", &viewEvent))
			}
		}

		// last tracks
		{
			if err := svc.db.
				Model(&sgtmpb.Post{}).
				Preload("Author").
				Where(sgtmpb.Post{
					Kind:       sgtmpb.Post_TrackKind,
					Visibility: sgtmpb.Visibility_Public,
				}).
				Order("sort_date desc").
				Limit(50). // FIXME: pagination
				Find(&data.Home.LastTracks).
				Error; err != nil {
				data.Error = "Cannot fetch last tracks: " + err.Error()
			}
			for _, track := range data.Home.LastTracks {
				track.ApplyDefaults()
			}
		}

		// last users
		{
			if err := svc.db.Model(&sgtmpb.User{}).
				Order("created_at desc").
				Limit(10).
				Find(&data.Home.LastUsers).
				Error; err != nil {
				data.Error = "Cannot fetch last users: " + err.Error() // FIXME: use slice instead of string
			}
			for _, user := range data.Home.LastUsers {
				user.ApplyDefaults()
			}
		}
		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "base.tmpl.html", "home.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.ExecuteTemplate(w, "base", &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) rssPage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplates(box, "rss.tmpl.xml")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom
		w.Header().Add("Content-Type", "application/xml")
		// last tracks
		{
			if err := svc.db.
				Model(&sgtmpb.Post{}).
				Preload("Author").
				Where(sgtmpb.Post{
					Kind:       sgtmpb.Post_TrackKind,
					Visibility: sgtmpb.Visibility_Public,
				}).
				Order("sort_date desc").
				Limit(50). // FIXME: pagination
				Find(&data.RSS.LastTracks).
				Error; err != nil {
				svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			}
			for _, track := range data.Home.LastTracks {
				track.ApplyDefaults()
			}
		}
		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "rss.tmpl.xml")
		}
		data.Duration = time.Since(started)
		if err := tmpl.ExecuteTemplate(w, "base", &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) settingsPage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplates(box, "base.tmpl.html", "settings.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom
		if data.User == nil {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		if r.Method == "POST" {
			validate := func() *sgtmpb.User {
				if err := r.ParseForm(); err != nil {
					data.Error = err.Error()
					return nil
				}
				// FIXME: blacklist, etc
				fields := sgtmpb.User{
					Firstname: r.Form.Get("firstname"),
					Lastname:  r.Form.Get("lastname"),
				}
				return &fields
			}
			fields := validate()
			if fields != nil {
				if err := svc.db.Model(data.User).Updates(fields).Error; err != nil {
					svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
					return
				}
				svc.logger.Debug("settings update", zap.Any("fields", fields))
				http.Redirect(w, r, "/settings", http.StatusFound)
				return
			}
		}
		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "base.tmpl.html", "settings.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.Execute(w, &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) openPage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplates(box, "base.tmpl.html", "open.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom

		// tracking
		{
			viewEvent := sgtmpb.Post{AuthorID: data.UserID, Kind: sgtmpb.Post_ViewOpenKind}
			if err := svc.db.Create(&viewEvent).Error; err != nil {
				data.Error = "Cannot write activity: " + err.Error()
			} else {
				svc.logger.Debug("new view open", zap.Any("event", &viewEvent))
			}
		}

		// public tracks
		{
			err := svc.db.
				Model(&sgtmpb.Post{}).
				Where(sgtmpb.Post{
					Kind:       sgtmpb.Post_TrackKind,
					Visibility: sgtmpb.Visibility_Public,
				}).
				Count(&data.Open.Tracks).
				Error
			if err != nil {
				data.Error = "Cannot fetch last tracks: " + err.Error()
			}
		}

		// tracks' duration
		{
			var result struct {
				TotalDuration int64
			}
			err := svc.db.
				Model(&sgtmpb.Post{}).
				Select("sum(duration) as total_duration").
				Where(sgtmpb.Post{
					Kind:       sgtmpb.Post_TrackKind,
					Visibility: sgtmpb.Visibility_Public,
				}).
				First(&result).
				Error
			if err != nil {
				data.Error = "Cannot fetch last track durations: " + err.Error()
			}
			data.Open.TotalDuration = time.Duration(result.TotalDuration) * time.Millisecond
		}

		// uploads by weekday
		{
			type result struct {
				Weekday  int64
				Quantity int64
			}
			var results []result
			err := svc.db.Model(&sgtmpb.Post{}).
				Where(&sgtmpb.Post{Kind: sgtmpb.Post_TrackKind}).
				Select(`strftime("%w", sort_date/1000000000, "unixepoch") as weekday , count(*) as quantity`).
				Group("weekday").Find(&results).
				Error
			if err != nil {
				data.Error = "Cannot fetch uploads by weekday: " + err.Error()
			}
			data.Open.UploadsByWeekday = make([]int64, 7)
			for _, result := range results {
				data.Open.UploadsByWeekday[result.Weekday] = result.Quantity
			}
		}

		// last activities
		{
			err := svc.db.
				Preload("Author").
				Preload("TargetPost").
				Preload("TargetUser").
				Order("created_at desc").
				Where("NOT (author_id == ? AND kind IN (?))", moulID, []sgtmpb.Post_Kind{
					sgtmpb.Post_ViewHomeKind,
					sgtmpb.Post_ViewOpenKind,
				}).
				Where("author_id != 0").
				Where("kind NOT IN (?)", []sgtmpb.Post_Kind{
					sgtmpb.Post_LinkDiscordAccountKind,
					//sgtmpb.Post_LoginKind,
				}).
				Limit(50).
				Find(&data.Open.LastActivities).
				Error
			if err != nil {
				data.Error = "Cannot fetch last activities: " + err.Error()
			}
		}

		// track drafts
		{
			err := svc.db.
				Model(&sgtmpb.Post{}).
				Where(sgtmpb.Post{
					Kind:       sgtmpb.Post_TrackKind,
					Visibility: sgtmpb.Visibility_Draft,
				}).
				Count(&data.Open.TrackDrafts).
				Error
			if err != nil {
				data.Error = "Cannot fetch last track drafts: " + err.Error()
			}
		}
		// users
		{
			err := svc.db.
				Model(&sgtmpb.User{}).
				Count(&data.Open.Users).
				Error
			if err != nil {
				data.Error = "Cannot fetch last users: " + err.Error()
			}
		}
		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "base.tmpl.html", "open.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.Execute(w, &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) newPage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplates(box, "base.tmpl.html", "new.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom
		if data.User == nil {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		if r.Method == "POST" {
			validate := func() *sgtmpb.Post {
				if err := r.ParseForm(); err != nil {
					data.Error = err.Error()
					return nil
				}
				if r.Form.Get("url") == "" {
					data.New.URLInvalidMsg = "Please specify a track link."
					return nil
				}
				data.New.URLValue = r.Form.Get("url")

				// FIXME: check if valid SoundCloud link
				post := sgtmpb.Post{
					Kind:       sgtmpb.Post_TrackKind,
					Visibility: sgtmpb.Visibility_Public,
					AuthorID:   data.User.ID,
					Slug:       "",
					Title:      "",
					SortDate:   time.Now().UnixNano(),
				}

				u, err := url.Parse(r.Form.Get("url"))
				if err != nil {
					data.Error = fmt.Sprintf("Parse URL: %s", err.Error())
					return nil
				}
				switch u.Host {
				case "soundcloud.com":
					sc := soundcloud.Api{ClientId: svc.opts.SoundCloudClientID}
					u, err := sc.Resolve(u.String())
					if err != nil {
						data.New.URLInvalidMsg = "This URL does not exist on SoundCloud.com."
						return nil
					}
					re := regexp.MustCompile(`/tracks/(.*).json`)
					matches := re.FindStringSubmatch(u.Path)
					if len(matches) != 2 {
						data.New.URLInvalidMsg = "Invalid SoundCloud track link."
						return nil
					}
					post.SoundCloudKind = sgtmpb.Post_SoundCloudTrack
					post.SoundCloudID, err = strconv.ParseUint(matches[1], 10, 64)
					if err != nil {
						data.New.URLInvalidMsg = fmt.Sprintf("Parse track ID: %s.", err.Error())
						return nil
					}

					post.SoundCloudSecretToken = u.Query().Get("secret_token")
					params := url.Values{}
					if post.SoundCloudSecretToken != "" {
						params.Set("secret_token", post.SoundCloudSecretToken)
					}
					track, err := sc.Track(post.SoundCloudID).Get(params)
					if err != nil {
						data.New.URLInvalidMsg = fmt.Sprintf("Fetch track info from SoundCloud: %s.", err.Error())
						return nil
					}

					post.ProviderMetadata = godev.JSON(track)
					post.Title = track.Title
					createdAt, err := time.Parse("2006/01/02 15:04:05 +0000", track.CreatedAt)
					if err == nil {
						post.ProviderCreatedAt = createdAt.UnixNano()
						post.SortDate = createdAt.UnixNano()
					}
					post.Genre = track.Genre
					post.Duration = track.Duration
					post.ArtworkURL = track.ArtworkUrl
					post.ISRC = track.ISRC
					post.BPM = track.Bpm
					post.KeySignature = track.KeySignature
					post.ProviderDescription = track.Description
					post.Body = track.Description
					/*post.Tags = track.TagList
					post.WaveformURL = track.WaveformURL
					post.License = track.License
					track.User*/
					if track.Downloadable {
						post.DownloadURL = track.DownloadUrl
					}
					post.URL = track.PermalinkUrl
					post.Provider = sgtmpb.Provider_SoundCloud
				default:
					data.New.URLInvalidMsg = fmt.Sprintf("Unsupported provider: %s.", html.EscapeString(u.Host))
					return nil
				}

				if r.Form.Get("submit") == "draft" {
					post.Visibility = sgtmpb.Visibility_Draft
				}
				return &post
			}
			post := validate()
			if post != nil {
				if err := svc.db.Create(&post).Error; err != nil {
					svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
					return
				}
				svc.logger.Debug("new post", zap.Any("post", post))
				http.Redirect(w, r, post.CanonicalURL(), http.StatusFound)
				return
			}
		}
		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "base.tmpl.html", "new.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.Execute(w, &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) error404Page(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplates(box, "base.tmpl.html", "error404.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom
		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "base.tmpl.html", "error404.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.Execute(w, &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) errorPage(box *packr.Box) func(w http.ResponseWriter, r *http.Request, err error, status int) {
	tmpl := loadTemplates(box, "base.tmpl.html", "error.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request, userError error, status int) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRender(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom
		w.WriteHeader(status)
		if userError != nil {
			data.Error = userError.Error()
		}
		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "base.tmpl.html", "error.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.Execute(w, &data); err != nil {
			svc.errRender(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) profilePage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplates(box, "base.tmpl.html", "profile.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom

		// load profile
		{
			userSlug := chi.URLParam(r, "user_slug")
			var user sgtmpb.User
			if err := svc.db.
				Where(sgtmpb.User{Slug: userSlug}).
				First(&user).
				Error; err != nil {
				svc.error404Page(box)(w, r)
				return
			}
			data.Profile.User = &user
		}

		// tracking
		{
			viewEvent := sgtmpb.Post{AuthorID: data.UserID, Kind: sgtmpb.Post_ViewProfileKind, TargetUserID: data.Profile.User.ID}
			if err := svc.db.Create(&viewEvent).Error; err != nil {
				data.Error = "Cannot write activity: " + err.Error()
			} else {
				svc.logger.Debug("new view profile", zap.Any("event", &viewEvent))
			}
		}

		// tracks
		{
			query := svc.db.
				Model(&sgtmpb.Post{}).
				Where(sgtmpb.Post{
					AuthorID:   data.Profile.User.ID,
					Kind:       sgtmpb.Post_TrackKind,
					Visibility: sgtmpb.Visibility_Public,
				})
			if err := query.Count(&data.Profile.Stats.Tracks).Error; err != nil {
				data.Error = "Cannot fetch last tracks: " + err.Error()
			}
			if data.Profile.Stats.Tracks > 0 {
				if err := query.
					Order("sort_date desc").
					Limit(50). // FIXME: pagination
					Find(&data.Profile.LastTracks).
					Error; err != nil {
					data.Error = "Cannot fetch last tracks: " + err.Error()
				}
			}
			for _, track := range data.Profile.LastTracks {
				track.ApplyDefaults()
			}
		}

		// calendar heatmap
		if data.Profile.Stats.Tracks > 0 {
			timestamps := []int64{}
			err := svc.db.Model(&sgtmpb.Post{}).
				Select(`sort_date/1000000000 as timestamp`).
				Where(sgtmpb.Post{
					AuthorID:   data.Profile.User.ID,
					Kind:       sgtmpb.Post_TrackKind,
					Visibility: sgtmpb.Visibility_Public,
				}).
				Pluck("timestamp", &timestamps).
				Error
			if err != nil {
				data.Error = "Cannot fetch post timestamps: " + err.Error()
			}
			data.Profile.CalendarHeatmap = map[int64]int64{}
			for _, timestamp := range timestamps {
				data.Profile.CalendarHeatmap[timestamp] = 1
			}
		}

		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "base.tmpl.html", "profile.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.Execute(w, &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) postPage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplates(box, "base.tmpl.html", "post.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom
		postSlug := chi.URLParam(r, "post_slug")
		query := svc.db.Preload("Author")
		id, err := strconv.ParseInt(postSlug, 10, 64)
		if err == nil {
			query = query.Where(sgtmpb.Post{ID: id, Kind: sgtmpb.Post_TrackKind})
		} else {
			query = query.Where(sgtmpb.Post{Slug: postSlug, Kind: sgtmpb.Post_TrackKind})
		}
		var post sgtmpb.Post
		if err := query.First(&post).Error; err != nil {
			svc.error404Page(box)(w, r)
			return
		}
		data.Post.Post = &post
		data.Post.Post.ApplyDefaults()

		// tracking
		{
			viewEvent := sgtmpb.Post{AuthorID: data.UserID, Kind: sgtmpb.Post_ViewPostKind, TargetPostID: data.Post.Post.ID}
			if err := svc.db.Create(&viewEvent).Error; err != nil {
				data.Error = "Cannot write activity: " + err.Error()
			} else {
				svc.logger.Debug("new view post", zap.Any("event", &viewEvent))
			}
		}

		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "base.tmpl.html", "post.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.Execute(w, &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) postSyncPage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplates(box, "base.tmpl.html", "dummy.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom
		if data.User == nil {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		postSlug := chi.URLParam(r, "post_slug")
		query := svc.db.Preload("Author")
		id, err := strconv.ParseInt(postSlug, 10, 64)
		if err == nil {
			query = query.Where(sgtmpb.Post{ID: id, Kind: sgtmpb.Post_TrackKind})
		} else {
			query = query.Where(sgtmpb.Post{Slug: postSlug, Kind: sgtmpb.Post_TrackKind})
		}
		var post sgtmpb.Post
		if err := query.First(&post).Error; err != nil {
			svc.error404Page(box)(w, r)
			return
		}
		if !data.IsAdmin && data.User.ID != post.Author.ID {
			svc.error404Page(box)(w, r)
			return
		}

		// FIXME: do the sync here

		http.Redirect(w, r, post.CanonicalURL(), http.StatusFound)
		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "base.tmpl.html", "dummy.tmpl.html")
		}
		data.Duration = time.Since(started)
		if err := tmpl.Execute(w, &data); err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
	}
}

func (svc *Service) postEditPage(box *packr.Box) func(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplates(box, "base.tmpl.html", "post-edit.tmpl.html")
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		data, err := svc.newTemplateData(r)
		if err != nil {
			svc.errRenderHTML(w, r, err, http.StatusUnprocessableEntity)
			return
		}
		// custom
		if data.User == nil {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		postSlug := chi.URLParam(r, "post_slug")
		query := svc.db.Preload("Author")
		id, err := strconv.ParseInt(postSlug, 10, 64)
		if err == nil {
			query = query.Where(sgtmpb.Post{ID: id, Kind: sgtmpb.Post_TrackKind})
		} else {
			query = query.Where(sgtmpb.Post{Slug: postSlug, Kind: sgtmpb.Post_TrackKind})
		}
		var post sgtmpb.Post
		if err := query.First(&post).Error; err != nil {
			svc.error404Page(box)(w, r)
			return
		}
		data.PostEdit.Post = &post
		if !data.IsAdmin && data.User.ID != post.Author.ID {
			svc.error404Page(box)(w, r)
			return
		}
		// end of custom
		if svc.opts.DevMode {
			tmpl = loadTemplates(box, "base.tmpl.html", "post-edit.tmpl.html")
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
		Title:   "SGTM",
		Date:    time.Now(),
		Opts:    svc.opts.Filtered(),
		Lang:    "en", // FIXME: dynamic
		Request: r,
		Service: svc,
	}
	if svc.opts.DevMode {
		data.Title += " (dev)"
	}

	if cookie, err := r.Cookie(oauthTokenCookie); err == nil {
		data.JWTToken = cookie.Value
		var err error
		data.Claims, err = svc.parseJWTToken(data.JWTToken)
		if err != nil {
			return nil, fmt.Errorf("parse jwt token: %w", err)
		}
		var user sgtmpb.User
		if err := svc.db.
			Preload("RecentPosts", func(db *gorm.DB) *gorm.DB {
				return db.Order("created_at desc").Limit(3)
			}).
			First(&user, data.Claims.Session.UserID).
			Error; err != nil {
			svc.logger.Warn("load user from DB", zap.Error(err))
		}
		data.User = &user
		data.UserID = user.ID
		data.IsAdmin = user.ID == 1280639244955553792
	}

	return &data, nil
}

func loadTemplates(box *packr.Box, filenames ...string) *template.Template {
	allInOne := ""
	templateName := ""
	for _, filename := range filenames {
		src, err := box.FindString("_layouts/" + filename)
		if err != nil {
			panic(err)
		}
		allInOne += strings.TrimSpace(src) + "\n"
		templateName += filename
	}
	allInOne = strings.TrimSpace(allInOne)
	funcmap := sprig.FuncMap()
	funcmap["fromUnixNano"] = func(input int64) time.Time {
		return time.Unix(0, input)
	}
	funcmap["prettyAgo"] = func(input time.Time) string {
		return humanize.RelTime(input, time.Now(), "ago", "in the future(!?)")
	}
	funcmap["prettyDuration"] = func(input time.Duration) string {
		input = input.Round(time.Second)
		str := durafmt.Parse(input).LimitFirstN(2).String()
		str = strings.Replace(str, " ", "", -1)
		str = strings.Replace(str, "minutes", "m", -1)
		str = strings.Replace(str, "minute", "m", -1)
		str = strings.Replace(str, "hours", "h", -1)
		str = strings.Replace(str, "hour", "h", -1)
		str = strings.Replace(str, "seconds", "s", -1)
		str = strings.Replace(str, "second", "s", -1)

		return str
	}
	funcmap["prettyDate"] = func(input time.Time) string {
		return input.Format("2006-01-02 15:04")
	}
	funcmap["noescape"] = func(str string) template.HTML {
		return template.HTML(str)
	}
	funcmap["escape"] = func(str string) template.HTML {
		return template.HTML(str)
	}
	funcmap["stripTags"] = striptags.StripTags
	funcmap["urlencode"] = url.PathEscape
	funcmap["plus1"] = func(x int) int {
		return x + 1
	}
	tmpl, err := template.New(templateName).Funcs(funcmap).Parse(allInOne)
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
	IsAdmin  bool
	User     *sgtmpb.User
	UserID   int64
	Error    string
	Service  *Service      `json:"-"`
	Request  *http.Request `json:"-"`

	// specific

	RSS struct {
		LastTracks []*sgtmpb.Post
	}
	Home struct {
		LastTracks []*sgtmpb.Post
		LastUsers  []*sgtmpb.User
	} `json:"Home,omitempty"`
	Settings struct {
	} `json:"Settings,omitempty"`
	Profile struct {
		User       *sgtmpb.User
		LastTracks []*sgtmpb.Post
		Stats      struct {
			Tracks int64
			// Drafts int64
		}
		CalendarHeatmap map[int64]int64
	} `json:"Profile,omitempty"`
	Open struct {
		Users            int64
		Tracks           int64
		TrackDrafts      int64
		TotalDuration    time.Duration
		UploadsByWeekday []int64
		LastActivities   []*sgtmpb.Post
	} `json:"Open,omitempty"`
	New struct {
		URLValue      string
		URLInvalidMsg string
	} `json:"New,omitempty"`
	Post struct {
		Post *sgtmpb.Post
	} `json:"Post,omitempty"`
	PostEdit struct {
		Post *sgtmpb.Post
	} `json:"PostEdit,omitempty"`
}
