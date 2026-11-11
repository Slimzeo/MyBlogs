package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type personalProject struct {
	Slug, Name, Repository, Summary, Notes string
}

var personalProjects = []personalProject{
	{
		Slug: "seekino", Name: "Seekino", Repository: "https://github.com/owariband/Seekino",
		Summary: "用一句描述或一张截图，在自己的视频库里找到课程或动画片段，直接跳回原片的对应时间点。",
		Notes: `## 找回记得的那一幕

想重看动画里的一场演出，手上可能只有一张截图。找课上的一段讲解，也常常只记得老师提过的几个词。把视频加入 Seekino 的素材库并解析一次，之后就能用图片或文字反复搜索。

![Seekino 的片段检索结果，右侧可播放带起止时间的原片](/user/img/projects/seekino-search.webp)

## 从结果回到原片

每条结果都标出来自哪部视频、从哪里开始、到哪里结束。可以先在结果旁预览，再跳回整集看前后文。观看课程时也可以围绕当前片段继续提问。`,
	},
	{
		Slug: "catendar", Name: "catendar", Repository: "https://github.com/Slimzeo/catendar",
		Summary: "把每日看板和月历放在一起的桌面应用。没做完的任务会延续到第二天，也能从邮件和 Codex 对话中整理待办与日程。",
		Notes: `## 打开就是今天的看板

把今天要做的事写成卡片，拖到「进行中」或「已完成」就能更新进度。同一列里，越靠上的卡片优先级越高。昨天没做完的任务会带到今天，已经完成的留在原来的日期。

![catendar 的每日看板，任务按待办、进行中和已完成分为三列](/user/img/projects/catendar-board.webp)

## 给有时间的事留个位置

会议和面试可以填起止时间，切到月历就能看见安排。任务存在本机。需要整理时，可以手动运行 AI Sync，从最近的 Codex 对话里汇总进展，或从邮箱提取日程。`,
	},
}

type projectSlide struct {
	personalProject
	Previous, Next string
}

func (server *Server) projects(context *gin.Context) {
	data := server.baseData(context, "个人项目", "projects")
	data.Description = "Hypnos 的个人项目简介与项目地址。"
	for i, project := range personalProjects {
		size := len(personalProjects)
		data.ProjectSlides = append(data.ProjectSlides, projectSlide{
			personalProject: project,
			Previous:        personalProjects[(i+size-1)%size].Slug,
			Next:            personalProjects[(i+1)%size].Slug,
		})
	}
	server.render(context, http.StatusOK, "projects", data)
}

func (server *Server) projectNotes(context *gin.Context) {
	for _, project := range personalProjects {
		if project.Slug == context.Param("slug") {
			data := server.baseData(context, project.Name, "projects")
			data.Description = project.Summary
			data.Project = &project
			server.render(context, http.StatusOK, "project-notes", data)
			return
		}
	}
	server.render(context, http.StatusNotFound, "error_404", PageData{})
}
