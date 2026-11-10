package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type blogLibrary struct {
	Category, Tag string
}

func directoryURL(path, category, tag string, page, size int) string {
	query := url.Values{}
	if category != "" {
		query.Set("category", category)
	}
	if tag != "" {
		query.Set("tag", tag)
	}
	if page > 1 {
		query.Set("page", strconv.Itoa(page))
	}
	if size != journalPageSize {
		query.Set("limit", strconv.Itoa(size))
	}
	return (&url.URL{Path: path, RawQuery: query.Encode()}).String()
}

func libraryURL(category, tag string, page int) string {
	path := "/categories"
	if category == "" && tag != "" {
		path = "/tags"
	}
	return directoryURL(path, category, tag, page, journalPageSize)
}

func (data PageData) DirectoryPageURL(page int) string {
	path := "/" + data.Active
	if data.Active == "search" {
		path += "/" + data.Keyword
	}
	var filters blogLibrary
	if data.Library != nil {
		filters = *data.Library
	}
	return directoryURL(path, filters.Category, filters.Tag, page, data.Articles.PageSize)
}

// Categories and Tags each have an index. A tag can also narrow an author's column.
func (server *Server) library(context *gin.Context) {
	category := strings.TrimSpace(context.Query("category"))
	tag := strings.TrimSpace(context.Query("tag"))
	page := clampPage(queryInt(context, "page", 1))
	tagIndex := context.FullPath() == "/tags"
	if tagIndex && category != "" || !tagIndex && category == "" && tag != "" {
		context.Redirect(http.StatusFound, libraryURL(category, tag, page))
		return
	}
	title, active := "文章专栏", "categories"
	if tagIndex {
		title, active = "标签", "tags"
	}
	data := server.baseData(context, title, active)
	data.Library = &blogLibrary{Category: category, Tag: tag}
	access := server.canAccessEncrypted(context)
	if category == "" && tag == "" {
		if page > 1 {
			context.Redirect(http.StatusFound, "/"+active)
			return
		}
		var err error
		if tagIndex {
			data.Tags, err = server.service.BlogTags("", access)
		} else {
			data.TopicGroups, err = server.service.GetPublishedTopicGroups(5, access)
		}
		if err != nil {
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		server.render(context, http.StatusOK, "directory", data)
		return
	}
	if category != "" {
		data.Title = category
		var err error
		data.Tags, err = server.service.BlogTags(category, access)
		if err != nil {
			context.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	} else {
		data.Title = "#" + tag
	}
	server.directoryPosts(context, data, category, tag, page)
}

func (server *Server) directoryPosts(context *gin.Context, data PageData, category, tag string, page int) {
	limit := clampLimit(queryInt(context, "limit", journalPageSize), journalPageSize, 100)
	posts, err := server.service.BlogArticles(category, tag, page, limit, server.canAccessEncrypted(context))
	if err != nil {
		context.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if page > posts.Pages {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	data.Articles = posts
	server.render(context, http.StatusOK, "directory", data)
}

// Existing /notes, /topics and filtered /blogs bookmarks keep their selections.
func (server *Server) topics(context *gin.Context) {
	category := strings.TrimSpace(context.Query("category"))
	tag := strings.TrimSpace(context.Query("tag"))
	if category == "" && tag == "" && context.Query("view") == "tags" {
		context.Redirect(http.StatusFound, "/tags")
		return
	}
	context.Redirect(http.StatusFound, libraryURL(category, tag, clampPage(queryInt(context, "page", 1))))
}
