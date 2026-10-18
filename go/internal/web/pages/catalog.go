package pages

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"

	guideview "github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/pages"
)

// Catalog limits match server/api/plan-routes.js and shared/guides-page.js.
const (
	catalogLimit   = 200
	sectionLimit   = 12
	paragraphLimit = 8
	exampleLimit   = 8
	calloutLimit   = 6
	columnLimit    = 8
	rowLimit       = 12
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ErrNotFound is a guide the reader cannot see, or a lesson that is not a guide.
var ErrNotFound = errors.New("guide not found")

// ErrMissingID is a plan id that is not a slug.
var ErrMissingID = errors.New("missing plan id")

// User is the signed-in reader. Nil means the temporary unauthenticated mount:
// personal plans and selected lessons stay hidden.
type User struct {
	ID       int64
	Username string
}

type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		*f = ""
		return nil
	}
	*f = flexString(s)
	return nil
}

func (f flexString) clip(n int) string {
	return guideview.ClipJS(string(f), n)
}

type snapshotDoc struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	Audience   string     `json:"audience"`
	AssigneeID flexString `json:"assigneeId"`
	Units      []unitDoc  `json:"units"`
}

type unitDoc struct {
	ID      string      `json:"id"`
	Title   flexString  `json:"title"`
	Lessons []lessonDoc `json:"lessons"`
}

type lessonDoc struct {
	ID         string     `json:"id"`
	Title      flexString `json:"title"`
	Recipe     string     `json:"recipe"`
	Visibility string     `json:"visibility"`
	Blocks     []blockDoc `json:"blocks"`
}

type blockDoc struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Title    flexString   `json:"title"`
	Sections []sectionDoc `json:"sections"`
}

type sectionDoc struct {
	ID         string       `json:"id"`
	Heading    flexString   `json:"heading"`
	Paragraphs []flexString `json:"paragraphs"`
	Examples   []exampleDoc `json:"examples"`
	Table      *tableDoc    `json:"table"`
	Callouts   []calloutDoc `json:"callouts"`
}

type exampleDoc struct {
	ID       flexString `json:"id"`
	He       flexString `json:"he"`
	En       flexString `json:"en"`
	Translit flexString `json:"translit"`
	Root     flexString `json:"root"`
}

type tableDoc struct {
	Columns []flexString   `json:"columns"`
	Rows    [][]flexString `json:"rows"`
}

type calloutDoc struct {
	ID    flexString `json:"id"`
	Tone  string     `json:"tone"`
	Title flexString `json:"title"`
	Text  flexString `json:"text"`
}

type catalogLesson struct {
	PlanID   string
	LessonID string
	Topic    string
	Title    string
	Lesson   lessonDoc
}

func isSlug(s string) bool {
	return slugPattern.MatchString(s)
}

func take[T any](in []T, n int) []T {
	if n < 0 {
		return nil
	}
	if len(in) > n {
		return in[:n]
	}
	return in
}

// Guides builds the shelf the Node /guides/ page paints.
// Text is clipped and escaped later by the template. It is not normalized.
func Guides(ctx context.Context, src Source, user *User) (guideview.GuideList, error) {
	rows, err := visibleGuides(ctx, src, user)
	if err != nil {
		return guideview.GuideList{}, err
	}
	groups := groupTopics(rows)
	list := guideview.GuideList{Topics: groups, Count: countCards(groups)}
	// The featured guide is the first card. It is not repeated in the grid.
	if list.Count >= 3 && len(groups) > 0 && len(groups[0].Cards) > 0 {
		hero := groups[0].Cards[0]
		list.Hero = &hero
		groups[0].Cards = groups[0].Cards[1:]
		if len(groups[0].Cards) == 0 {
			groups = groups[1:]
		}
		list.Topics = groups
	}
	return list, nil
}

func countCards(groups []guideview.TopicGroup) int {
	n := 0
	for _, group := range groups {
		n += len(group.Cards)
	}
	return n
}

// OpenGuide returns one guide that would appear on the shelf.
func OpenGuide(ctx context.Context, src Source, user *User, planID, lessonID string) (guideview.GuideDetail, error) {
	if !isSlug(planID) {
		return guideview.GuideDetail{}, ErrMissingID
	}
	if !isSlug(lessonID) {
		return guideview.GuideDetail{}, ErrNotFound
	}
	rows, err := visibleGuides(ctx, src, user)
	if err != nil {
		return guideview.GuideDetail{}, err
	}
	for _, row := range rows {
		if row.PlanID != planID || row.LessonID != lessonID {
			continue
		}
		topic := row.Topic
		if topic == "" {
			topic = "Guides"
		}
		lang, dir := guideview.LangDir(row.Title)
		topicLang, topicDir := guideview.LangDir(topic)
		return guideview.GuideDetail{
			PlanID:    row.PlanID,
			LessonID:  row.LessonID,
			Title:     row.Title,
			Lang:      lang,
			Dir:       dir,
			Topic:     topic,
			TopicLang: topicLang,
			TopicDir:  topicDir,
			Blocks:    explainerBlocks(row.Lesson.Blocks),
		}, nil
	}
	return guideview.GuideDetail{}, ErrNotFound
}

func visibleGuides(ctx context.Context, src Source, user *User) ([]catalogLesson, error) {
	if src == nil {
		return nil, errors.New("guides: no database")
	}
	snaps, err := src.Snapshots(ctx)
	if err != nil {
		return nil, err
	}
	var catalog []catalogLesson
	for _, snap := range snaps {
		if len(catalog) >= catalogLimit {
			break
		}
		if !isSlug(snap.ID) || snap.Snapshot == "" {
			continue
		}
		var doc snapshotDoc
		if err := json.Unmarshal([]byte(snap.Snapshot), &doc); err != nil {
			continue
		}
		if doc.ID != snap.ID {
			continue
		}
		ok, err := mayRead(ctx, src, user, doc)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		for _, unit := range doc.Units {
			if len(catalog) >= catalogLimit {
				break
			}
			topic := unit.Title.clip(guideview.CatalogTextLimit)
			for _, lesson := range unit.Lessons {
				if len(catalog) >= catalogLimit {
					break
				}
				if !isSlug(lesson.ID) {
					continue
				}
				if !knownRecipe(lesson.Recipe) {
					lesson.Recipe = ""
				}
				catalog = append(catalog, catalogLesson{
					PlanID:   snap.ID,
					LessonID: lesson.ID,
					Topic:    topic,
					Title:    lesson.Title.clip(guideview.CatalogTextLimit),
					Lesson:   lesson,
				})
			}
		}
	}
	var guides []catalogLesson
	for _, row := range catalog {
		if len(guides) >= catalogLimit {
			break
		}
		if row.Lesson.Recipe != "guide" {
			continue
		}
		if !isSlug(row.PlanID) || !isSlug(row.LessonID) {
			continue
		}
		guides = append(guides, row)
	}
	return guides, nil
}

func knownRecipe(recipe string) bool {
	switch recipe {
	case "topic-conversation", "review-worksheet", "verse-reading", "verb-workshop", "letters-and-vowels", "guide":
		return true
	default:
		return false
	}
}

// mayRead matches plan-routes.js mayReadPublished for a nil or real user.
func mayRead(ctx context.Context, src Source, user *User, doc snapshotDoc) (bool, error) {
	if doc.Status != "published" {
		return false, nil
	}
	if doc.Audience == "personal" {
		if user == nil || string(doc.AssigneeID) == "" {
			return false, nil
		}
		return string(doc.AssigneeID) == user.Username, nil
	}
	if doc.Audience != "shared" {
		return false, nil
	}
	lessons := 0
	for _, unit := range doc.Units {
		for _, lesson := range unit.Lessons {
			lessons++
			if lesson.ID == "" {
				return false, nil
			}
			switch lesson.Visibility {
			case "everyone":
				continue
			case "selected":
				if user == nil {
					return false, nil
				}
				ok, err := src.HasGrant(ctx, lesson.ID, user.ID)
				if err != nil {
					return false, err
				}
				if !ok {
					return false, nil
				}
			default:
				return false, nil
			}
		}
	}
	return lessons > 0, nil
}

func groupTopics(rows []catalogLesson) []guideview.TopicGroup {
	var groups []guideview.TopicGroup
	index := map[string]int{}
	for _, row := range rows {
		topic := row.Topic
		if topic == "" {
			topic = "Guides"
		}
		i, ok := index[topic]
		if !ok {
			lang, dir := guideview.LangDir(topic)
			index[topic] = len(groups)
			groups = append(groups, guideview.TopicGroup{Topic: topic, Lang: lang, Dir: dir})
			i = len(groups) - 1
		}
		lang, dir := guideview.LangDir(row.Title)
		groups[i].Cards = append(groups[i].Cards, guideview.GuideCard{
			PlanID:   row.PlanID,
			LessonID: row.LessonID,
			Title:    row.Title,
			Lang:     lang,
			Dir:      dir,
			Href:     planHref(row.PlanID, row.LessonID),
			Topic:    topic,
			Cover:    guideview.CoverFor(row.PlanID),
			Letter:   guideview.CoverLetter(row.Title, topic),
		})
	}
	guideview.AssignSlugs(groups)
	return groups
}

func planHref(planID, lessonID string) string {
	return "/shared/plan/?id=" + url.QueryEscape(planID) + "&lesson=" + url.QueryEscape(lessonID)
}

func explainerBlocks(blocks []blockDoc) []guideview.Block {
	var out []guideview.Block
	for _, block := range blocks {
		if block.Type != "explainer" {
			continue
		}
		view := guideview.Block{
			ID:    guideview.ClipJS(block.ID, guideview.ExplainerTextLimit),
			Title: block.Title.clip(guideview.ExplainerTextLimit),
		}
		for _, section := range take(block.Sections, sectionLimit) {
			sec := guideview.Section{
				ID:      guideview.ClipJS(section.ID, guideview.ExplainerTextLimit),
				Heading: section.Heading.clip(guideview.ExplainerTextLimit),
			}
			for _, paragraph := range take(section.Paragraphs, paragraphLimit) {
				sec.Paragraphs = append(sec.Paragraphs, paragraph.clip(guideview.ExplainerTextLimit))
			}
			for _, ex := range take(section.Examples, exampleLimit) {
				sec.Examples = append(sec.Examples, guideview.Example{
					ID:       ex.ID.clip(guideview.ExplainerTextLimit),
					He:       string(ex.He),
					En:       ex.En.clip(guideview.ExplainerTextLimit),
					Translit: ex.Translit.clip(guideview.ExplainerTextLimit),
					Root:     string(ex.Root),
				})
			}
			sec.Table = viewTable(section.Table)
			for _, c := range take(section.Callouts, calloutLimit) {
				sec.Callouts = append(sec.Callouts, guideview.Callout{
					ID:    c.ID.clip(guideview.ExplainerTextLimit),
					Tone:  c.Tone,
					Title: c.Title.clip(guideview.ExplainerTextLimit),
					Text:  c.Text.clip(guideview.ExplainerTextLimit),
				})
			}
			view.Sections = append(view.Sections, sec)
		}
		out = append(out, view)
	}
	return out
}

func viewTable(table *tableDoc) *guideview.Table {
	if table == nil || len(table.Columns) == 0 {
		return nil
	}
	cols := take(table.Columns, columnLimit)
	view := &guideview.Table{}
	for _, column := range cols {
		view.Columns = append(view.Columns, column.clip(guideview.ExplainerTextLimit))
	}
	for _, row := range take(table.Rows, rowLimit) {
		cells := make([]string, len(cols))
		for i := range cols {
			if i < len(row) {
				cells[i] = row[i].clip(guideview.ExplainerTextLimit)
			}
		}
		view.Rows = append(view.Rows, cells)
	}
	return view
}
