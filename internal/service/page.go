package service

// PageInfo mirrors the fields of MyBatis PageHelper's PageInfo that the
// templates rely on (list, navigation flags, navigate page numbers).
type PageInfo[T any] struct {
	List             []T   `json:"list"`
	PageNum          int   `json:"pageNum"`
	PageSize         int   `json:"pageSize"`
	Size             int   `json:"size"`
	Total            int64 `json:"total"`
	Pages            int   `json:"pages"`
	PrePage          int   `json:"prePage"`
	NextPage         int   `json:"nextPage"`
	IsFirstPage      bool  `json:"isFirstPage"`
	IsLastPage       bool  `json:"isLastPage"`
	HasPreviousPage  bool  `json:"hasPreviousPage"`
	HasNextPage      bool  `json:"hasNextPage"`
	NavigatePages    int   `json:"navigatePages"`
	NavigatepageNums []int `json:"navigatepageNums"`
}

// NewPageInfo builds pagination metadata from a page of results.
func NewPageInfo[T any](list []T, pageNum, pageSize int, total int64) *PageInfo[T] {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 1
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	p := &PageInfo[T]{
		List:          list,
		PageNum:       pageNum,
		PageSize:      pageSize,
		Size:          len(list),
		Total:         total,
		Pages:         pages,
		NavigatePages: 8,
	}
	p.IsFirstPage = pageNum <= 1
	p.IsLastPage = pageNum >= pages
	p.HasPreviousPage = pageNum > 1
	p.HasNextPage = pageNum < pages
	p.PrePage = pageNum - 1
	if p.PrePage < 1 {
		p.PrePage = 1
	}
	p.NextPage = pageNum + 1
	if p.NextPage > pages {
		p.NextPage = pages
	}
	p.NavigatepageNums = navPages(pageNum, pages, p.NavigatePages)
	return p
}

// navPages computes the window of page numbers shown in the pager, matching
// PageHelper's centered navigation behaviour.
func navPages(pageNum, pages, navCount int) []int {
	if pages <= 0 {
		return []int{}
	}
	if navCount > pages {
		navCount = pages
	}
	start := pageNum - (navCount-1)/2
	end := pageNum + navCount/2
	if start < 1 {
		start = 1
		end = navCount
	}
	if end > pages {
		end = pages
		start = pages - navCount + 1
		if start < 1 {
			start = 1
		}
	}
	nums := make([]int, 0, end-start+1)
	for i := start; i <= end; i++ {
		nums = append(nums, i)
	}
	return nums
}
