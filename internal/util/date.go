package util

import (
	"math/rand"
	"sync"
	"time"
)

// CurrentUnixTime mirrors DateKit.getCurrentUnixTime (seconds).
func CurrentUnixTime() int {
	return int(time.Now().Unix())
}

// goLayout converts a subset of Java SimpleDateFormat patterns to Go layouts.
// Only the patterns actually used by the templates/services are handled.
func goLayout(pattern string) string {
	repl := []struct{ from, to string }{
		{"yyyy", "2006"},
		{"MM", "01"},
		{"dd", "02"},
		{"HH", "15"},
		{"mm", "04"},
		{"ss", "05"},
	}
	out := pattern
	for _, r := range repl {
		out = replaceAll(out, r.from, r.to)
	}
	return out
}

func replaceAll(s, from, to string) string {
	res := ""
	for {
		i := indexOf(s, from)
		if i < 0 {
			return res + s
		}
		res += s[:i] + to
		s = s[i+len(from):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// FormatUnix formats a unix timestamp using a Java-style pattern.
// Mirrors DateKit.formatDateByUnixTime / Commons.fmtdate.
func FormatUnix(unixTime int, pattern string) string {
	if unixTime <= 0 || pattern == "" {
		return ""
	}
	return time.Unix(int64(unixTime), 0).Format(goLayout(pattern))
}

// FormatUnixCN formats a unix timestamp as "2006年01月" style Chinese month.
// Mirrors the FROM_UNIXTIME(created,'%Y年%m月') used for archives.
func FormatUnixCN(unixTime int) string {
	t := time.Unix(int64(unixTime), 0)
	return t.Format("2006年01月")
}

// ParseCNMonthRange returns [start,end] unix seconds spanning the month named
// like "2017年06月". Mirrors SiteServiceImpl.getArchives date math.
func ParseCNMonthRange(cn string) (int, int) {
	t, err := time.ParseInLocation("2006年01月", cn, time.Local)
	if err != nil {
		return 0, 0
	}
	start := t.Unix()
	end := t.AddDate(0, 1, 0).Unix() - 1
	return int(start), int(end)
}

var (
	seededRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
	seededRandMu sync.Mutex
)

// RandomNumber returns a string of `size` random digits (TaleUtils.getRandomNumber).
func RandomNumber(size int) string {
	const digits = "123456789"
	b := make([]byte, size)
	seededRandMu.Lock()
	defer seededRandMu.Unlock()
	for i := range b {
		b[i] = digits[seededRand.Intn(len(digits))]
	}
	return string(b)
}

// RandInt returns a random int in [min,max] (UUID.random).
func RandInt(min, max int) int {
	if max <= min {
		return min
	}
	seededRandMu.Lock()
	defer seededRandMu.Unlock()
	return seededRand.Intn(max-min+1) + min
}
