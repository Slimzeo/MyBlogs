package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"myblog/internal/model"
	"myblog/internal/service"

	"github.com/gin-gonic/gin"
)

type blogLibrary struct {
	Category, Tag, Description string
	Columns                    []service.BlogColumn
	Tags, MoreTags             []model.MetaDto
	ExpandTags                 bool
}

func libraryURL(category, tag string, page int) string {
	q := url.Values{}
	if category != "" {
		q.Set("category", category)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	if len(q) == 0 {
		return "/notes"
	}
	return "/notes?" + q.Encode()
}

func (server *Server) columns(context *gin.Context) {
	category, tag := strings.TrimSpace(context.Query("category")), strings.TrimSpace(context.Query("tag"))
	page := clampPage(queryInt(context, "page", 1))
	access := server.canAccessEncrypted(context)
	columns, err := server.service.BlogColumns(access)
	if err != nil {
		context.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	tags, err := server.service.BlogTags(category, access)
	if err != nil {
		context.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	lib := &blogLibrary{Category: category, Tag: tag, Columns: columns, Tags: tags, ExpandTags: context.Query("view") == "tags"}
	for _, col := range columns {
		if col.Name == category {
			lib.Description = col.Description
			break
		}
	}
	const shown = 7
	if len(tags) > shown {
		// A selected tag stays visible even after following a link from an article.
		for i := shown; i < len(tags); i++ {
			if tags[i].Name == tag {
				tags[shown-1], tags[i] = tags[i], tags[shown-1]
				break
			}
		}
		lib.Tags, lib.MoreTags = tags[:shown], tags[shown:]
	}
	data := server.baseData(context, "文章专栏", "notes")
	data.Library = lib
	if category != "" || tag != "" {
		data.Articles, err = server.service.BlogArticles(category, tag, page, journalPageSize, access)
		if err != nil {
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		if page > data.Articles.Pages {
			server.render(context, http.StatusNotFound, "error_404", PageData{})
			return
		}
		if category != "" {
			data.Title = category + " · 专栏"
		}
		if tag != "" {
			data.Title += " · #" + tag
		}
	} else if page != 1 {
		context.Redirect(http.StatusFound, "/notes")
		return
	}
	if server.renderReferencePage(context, data, "blog-library") {
		return
	}
	server.render(context, http.StatusOK, "blogs", data)
}

// Keep old bookmarks, including the old tag tab, on the same directory.
func (server *Server) topics(context *gin.Context) {
	target := libraryURL(context.Query("category"), context.Query("tag"), clampPage(queryInt(context, "page", 1)))
	if context.Query("view") == "tags" {
		u, _ := url.Parse(target)
		q := u.Query()
		q.Set("view", "tags")
		u.RawQuery = q.Encode()
		u.Fragment = "tags"
		target = u.String()
	}
	context.Redirect(http.StatusFound, target)
}
