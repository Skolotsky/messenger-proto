package democontent

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// The agents' routine work. Agents repeat a form on purpose, as real ones do; what
// varies is the facts, so no two reports say the same thing.

var customers = []string{
	"Fennimoor Dental", "Tallowbrook Family Practice", "Quillfield Physio", "Orrinvale Clinics",
	"Wexhollow Dental", "Marrowgate Eye Care", "Penhallow Pediatrics", "Cobbleford Health",
	"Birchwick Dental", "Rosmerry Physio", "Hethersett Vets",
}

var ticketProblems = []string{
	"double booking when two receptionists save the same slot within a second",
	"reminder texts going out an hour early since the clock change",
	"recurring appointment lost its end date after being moved",
	"CSV export cuts off the notes column at 255 characters",
	"new staff member can see the calendar but not the waiting list",
	"invoice per location shows the wrong VAT number on the second page",
	"patient search misses names with an apostrophe",
	"calendar is blank in Safari until the page is refreshed",
	"cancellation link in the email opens a 404 for one location",
	"waiting list does not offer the freed slot to the next patient",
	"the room filter resets every time the week changes",
	"online booking page shows a practitioner who left in September",
	"SMS credits were charged twice on the 1st",
	"mobile app logs out after every update",
	"drag and drop moves the appointment to the wrong day in the month view",
	"holiday closures are not blocking online bookings",
	"two-factor codes arrive after they have already expired",
	"the print view of the day list drops the last three rows",
	"a merged patient record still shows up twice in search",
	"opening hours import failed on a file with Windows line endings",
	"a patient with two surnames is sorted under the wrong letter",
	"the day list shows yesterday's room changes as if they were today's",
	"reminder wording reverted to the default after a settings save",
	"the location switcher hides the third clinic on small laptops",
	"a practitioner on leave still appears as bookable online",
	"cancelling one appointment of a series emailed the patient about all of them",
	"the waiting list count stays at one after the list has been emptied",
	"exported dates come out in American format",
	"a new patient's phone number loses its leading zero on import",
	"the weekly report arrives on Tuesday instead of Monday",
}

var ticketCategories = []string{"a known bug", "a configuration issue", "a training question", "a regression from the last beta", "a new problem", "a duplicate of last week's report"}

var services = []string{"api-gateway", "booking-write", "calendar-read", "reminder-worker", "sms-relay", "export-jobs", "search-index", "auth", "billing-sync", "web-edge"}

var alertShapes = []string{
	"p95 latency on %s is %d ms, threshold 800.",
	"%s error rate at %d per thousand for the last five minutes.",
	"%s queue depth is %d and rising.",
	"%s restarted %d times in the last 15 minutes.",
	"Disk on the %s node is at %d percent.",
}

var alertRecoveries = []string{
	"back under threshold after %d minutes. No action taken.",
	"recovered on its own in %d minutes; looks like the nightly job overlapping with morning traffic.",
	"cleared after %d minutes once the autoscaler added a second instance.",
	"stable again for %d minutes. Leaving the alert open until the next check passes.",
}

var buildBranches = []string{"main", "main", "main", "release/2.0", "release/2.0"}

var stopReasons = []string{
	"migration 04%d would hold a lock on appointments for about %d seconds",
	"contract check failed on the reminder payload (%d fields differ, %d of them required)",
	"bundle grew past the budget: main chunk is %d kB, limit %d kB",
	"smoke run failed on the waiting list flow in %d of %d browsers",
}

var findings = []string{
	"Of the pilot clinics, the ones with more than three rooms all keep a paper backup of the day list. The reason given is always the same: they do not trust the screen at 8 am.",
	"Receptionists switch between week and day view about 40 times a shift. Nobody I read mentioned it as a problem, but every recording shows it.",
	"Cancellations cluster in the 24 hours before the appointment. Clinics with a waiting list refill about half of them, clinics without refill almost none.",
	"Three competitors now advertise recurring appointments, and in all three the edit flow forces you to choose 'this one or all' before showing what would change.",
	"The word 'practitioner' tests badly with dental practices. They say dentist or hygienist, and two people asked what it meant.",
	"Owners and front desk want different things from the same dashboard. Owners open it weekly and look at revenue; front desk never opens it.",
	"Reminder timing: 48 hours plus 2 hours performs better than 24 hours alone in every study I could find, and no study found it annoying patients.",
	"Nearly all complaints about online booking come from the step where a patient has to pick a practitioner before seeing any times.",
	"Multi-location clinics treat 'location' as a filter, small ones treat it as noise. Hiding it below two locations would remove a control for 70 percent of accounts.",
	"Five interviewees described exporting to a spreadsheet every Friday to do one calculation: no-show rate by practitioner.",
	"Physios book in blocks of six to ten sessions and think in courses of treatment, not single appointments. Our model has no word for that.",
	"When a clinic switches from a competitor, the first week of support tickets is almost entirely about finding the same report under a different name.",
	"Patients on the waiting list respond to an offered slot within 20 minutes or not at all, so a longer hold mostly delays the next person.",
	"Small practices rarely change settings after the first week. Whatever we ship as a default is effectively the product for them.",
	"Clinics in the pilot print the day list mainly for the treatment rooms, where there is no screen. A phone view of 'my day' might replace most of it.",
	"Front desk staff in three clinics share one login, so anything we build around 'who did this' is unreliable for them until accounts are easier to create.",
}

var workItems = map[string][]string{
	"backend": {"the location index migration", "the streaming writer for XLSX", "retry logic in the reminder worker", "the booking conflict check",
		"the audit trail design", "rate limits on the public booking page", "the patient merge tool", "cancellation emails", "permissions for part-time staff", "the invoice per location template"},
	"frontend": {"the date picker focus states", "the empty states for the waiting list", "the calendar drag handles", "dark mode contrast on the day view",
		"the recurring appointment editor", "the print stylesheet", "bundle size of the calendar", "the import wizard for opening hours"},
	"mobile": {"Android 12 notification permissions", "session expiry on mobile", "offline banner on the tablet layout", "swipe between days", "the phone day view", "store listing screenshots"},
	"infra":  {"load test of the Monday morning peak", "the SMS provider failover", "the search reindex job", "backup timing", "the launch day runbook", "connection pool settings"},
	"qa":     {"regression cases for recurring appointments", "the clock change test run", "the waiting list test plan", "visual checks on the calendar", "the Monday traffic replay"},
}

func (b *builder) pick(list []string) string { return list[b.rng.IntN(len(list))] }

// next takes the next item of a shuffled pool, so a pool is used up before anything repeats.
func (b *builder) next(name string, pool []string) string {
	if len(b.decks[name]) == 0 {
		deck := slices.Clone(pool)
		b.rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		b.decks[name] = deck
	}
	item := b.decks[name][0]
	b.decks[name] = b.decks[name][1:]
	return item
}

// answerers are the people who would plausibly respond to each agent.
var answerers = map[string][]string{
	"ship":   {"marco", "yuki", "leila", "tomas", "ines", "camila", "kofi", "nadia", "arjun"},
	"watch":  {"felix", "bram", "marco", "arjun", "nadia", "tomas"},
	"triage": {"sam", "greta", "mateo", "ruth", "sofia"},
	"ledger": {"dele", "noor", "omar", "maya", "jonas", "owen", "ruth"},
	"field":  {"lena", "theo", "zoe", "maya", "dele", "noor", "ruth"},
	"scribe": {"tomas", "ines", "kofi", "nadia", "arjun", "camila", "hana", "felix", "bram", "yuki", "leila"},
}

func (b *builder) workdays() []time.Time {
	var out []time.Time
	for _, phase := range []string{"early", "mid", "late"} {
		out = append(out, b.days(phase)...)
	}
	return out
}

func (b *builder) say(at time.Time, author, conv string, root *planned, text, level string) *planned {
	p := &planned{at: at, author: author, conv: conv, root: root, text: text, level: level}
	b.addPost(p)
	return p
}

// humanAnswer sometimes has a member reply to an agent's post, from that agent's pool,
// using each line at most twice.
func (b *builder) humanAnswer(p *planned, chance float64) {
	pool := b.c.AgentReplies[p.author]
	if len(pool) == 0 || b.rng.Float64() >= chance {
		return
	}
	var people []string
	for _, id := range answerers[p.author] {
		if b.c.members[p.conv][id] {
			people = append(people, id)
		}
	}
	if len(people) == 0 {
		return
	}
	root := p
	if p.root != nil {
		root = p.root
	}
	text := b.fresh(pool)
	if text == "" {
		return
	}
	author := people[b.rng.IntN(len(people))]
	if ids, _ := b.c.mentionsIn(text); len(ids) > 0 && (ids[0] == author || !b.c.members[p.conv][ids[0]]) {
		return
	}
	b.say(p.at.Add(time.Duration(2+b.rng.IntN(40))*time.Minute), author, p.conv, root, text, "")
}

func (b *builder) agentTraffic() {
	beta, rc := 3, 0
	for _, day := range b.workdays() {
		late := day.Sub(b.start) >= 7*24*time.Hour
		moments := make([]time.Time, 2+b.rng.IntN(2))
		for i := range moments {
			moments[i] = b.workMoment(day, false)
		}
		slices.SortFunc(moments, time.Time.Compare)
		if late {
			rc++
		}
		for i, at := range moments {
			var version string
			switch {
			case !late:
				beta++
				version = fmt.Sprintf("2.0.0-beta.%d", beta)
			case i == 0:
				version = fmt.Sprintf("2.0.0-rc.%d", rc)
			default:
				version = fmt.Sprintf("2.0.0-rc.%d rebuild %d", rc, i)
			}
			b.buildRun(at, version)
		}
		b.alerts(day)
		b.tickets(day)
		b.metrics(day)
		b.research(day)
		b.standup(day)
	}
}

func (b *builder) buildRun(at time.Time, version string) {
	sha := fmt.Sprintf("%07x", b.rng.Uint32()&0xfffffff)
	branch := b.pick(buildBranches)
	start := fmt.Sprintf("Building %s from %s (%s).", version, sha, branch)
	root := b.say(at, "ship", "#releases", nil, start, "info")
	t := at.Add(time.Duration(2+b.rng.IntN(3)) * time.Minute)
	tests := 1380 + b.rng.IntN(160)
	b.say(t, "ship", "#releases", root, fmt.Sprintf("Unit and contract tests passed: %s in %dm %02ds.", thousands(tests), 3+b.rng.IntN(3), b.rng.IntN(60)), "info")
	t = t.Add(time.Duration(3+b.rng.IntN(4)) * time.Minute)
	if b.rng.Float64() < 0.5 {
		b.say(t, "ship", "#releases", root, fmt.Sprintf("Migration dry run on a copy of production: %d s, longest lock %d ms.", 12+b.rng.IntN(50), 40+b.rng.IntN(180)), "info")
		t = t.Add(time.Duration(2+b.rng.IntN(3)) * time.Minute)
	}
	minutes := int(t.Sub(at).Minutes()) + 3
	if b.rng.Float64() < 0.16 {
		shape := b.pick(stopReasons)
		var reason string
		switch {
		case strings.Contains(shape, "kB"):
			limit := 410 + b.rng.IntN(40)
			reason = fmt.Sprintf(shape, limit+3+b.rng.IntN(25), limit)
		case strings.Contains(shape, "browsers"):
			reason = fmt.Sprintf(shape, 1+b.rng.IntN(2), 3)
		case strings.Contains(shape, "migration"):
			reason = fmt.Sprintf(shape, 10+b.rng.IntN(30), 8+b.rng.IntN(30))
		default:
			reason = fmt.Sprintf(shape, 2+b.rng.IntN(4), 1)
		}
		stop := b.say(t, "ship", "#releases", root, fmt.Sprintf("@Marco I stopped %s before staging: %s. Retry, or hold the release?", version, reason), "flag")
		if answer := b.fresh(stopAnswers); answer != "" {
			b.say(stop.at.Add(time.Duration(4+b.rng.IntN(30))*time.Minute), "marco", "#releases", root, answer, "")
		}
		return
	}
	flows := 12 + b.rng.IntN(6)
	result := b.say(t, "ship", "#releases", root, fmt.Sprintf("%s is on staging. Smoke run clean across %d flows, %d minutes in total.", version, flows, minutes), "")
	if b.rng.Float64() < 0.8 {
		b.schedule(t.Add(20*time.Second), func() error {
			if root.m == nil {
				return nil
			}
			return b.w.edit(b.w.at, root.m, fmt.Sprintf("Built %s from %s (%s). On staging after %d minutes.", version, sha, branch, minutes), "same")
		})
	}
	b.humanAnswer(result, 0.3)
}

func (b *builder) alerts(day time.Time) {
	onCall := []string{"felix", "bram"}
	for range 4 + b.rng.IntN(5) {
		at := b.workMoment(day, false)
		service := b.pick(services)
		shape := b.pick(alertShapes)
		var n int
		switch {
		case strings.Contains(shape, "latency"):
			n = 820 + b.rng.IntN(900)
		case strings.Contains(shape, "error"):
			n = 4 + b.rng.IntN(30)
		case strings.Contains(shape, "queue"):
			n = 300 + b.rng.IntN(4000)
		case strings.Contains(shape, "restarted"):
			n = 2 + b.rng.IntN(5)
		default:
			n = 81 + b.rng.IntN(15)
		}
		text := at.Format("15:04") + " " + fmt.Sprintf(shape, service, n)
		if b.rng.Float64() < 0.14 {
			who := onCall[b.rng.IntN(2)]
			name := strings.Fields(b.c.people[who].Name)[0]
			root := b.say(at, "watch", "#incidents", nil, fmt.Sprintf("@%s %s It has not recovered in ten minutes, this needs a person.", name, text), "flag")
			if answer := b.fresh(onCallAnswers); answer != "" {
				b.say(at.Add(time.Duration(2+b.rng.IntN(9))*time.Minute), who, "#incidents", root, answer, "")
			}
			continue
		}
		root := b.say(at, "watch", "#incidents", nil, text, "info")
		b.say(at.Add(time.Duration(3+b.rng.IntN(20))*time.Minute), "watch", "#incidents", root,
			service+": "+fmt.Sprintf(b.pick(alertRecoveries), 3+b.rng.IntN(20)), "info")
		b.humanAnswer(root, 0.1)
	}
}

var stopAnswers = []string{
	"Hold it. I will look after standup.",
	"Retry once, I think that one is flaky.",
	"Hold. @Tomás can you take a look?",
	"Retry. If it fails again leave it for me.",
	"Hold, and do not retry until I say so. I want to read the diff first.",
	"That is a real one. Hold, I am pulling the commit out of the branch.",
	"Retry after 14:00, the staging database is busy with the rehearsal until then.",
	"Hold please. @Nadia this looks like your area.",
	"Fine to retry. It failed the same way on Tuesday and passed on the second go.",
}

var onCallAnswers = []string{
	"Connection count looks wrong to me, checking the pool first.",
	"Seen. It lines up with the import that started at the top of the hour.",
	"I am in the logs. Give me five minutes before anyone restarts anything.",
	"Failing over to the second instance while I find out why.",
	"Paged myself out of lunch for this one. Looking.",
	"Traffic is normal, so it is us and not a spike. Digging in.",
	"On it.",
	"Looking now. Probably the Monday batch again.",
	"Mine. Scaling it up by hand first, root cause after.",
	"I see it. Holding deploys until this settles.",
	"Taking this one. Will post what I find in the thread.",
	"Restarted the bad instance, watching it for ten minutes.",
}

func (b *builder) tickets(day time.Time) {
	n := 6 + b.rng.IntN(5)
	open := 18 + b.rng.IntN(25)
	b.say(day.Add(8*time.Hour+time.Duration(35+b.rng.IntN(15))*time.Minute), "triage", "#support", nil,
		fmt.Sprintf("Morning summary: %d tickets open, %d arrived overnight, %d answered from the help centre without a person.", open, n+b.rng.IntN(6), 2+b.rng.IntN(5)), "info")
	routed := []string{"greta", "mateo", "sam"}
	moments := make([]time.Time, n)
	for i := range moments {
		moments[i] = b.workMoment(day, false)
	}
	slices.SortFunc(moments, time.Time.Compare)
	for _, at := range moments {
		customer, problem := b.pick(customers), b.next("problems", ticketProblems)
		who := routed[b.rng.IntN(len(routed))]
		name := strings.Fields(b.c.people[who].Name)[0]
		b.ticket += 1 + b.rng.IntN(4)
		ticket := b.ticket
		if b.rng.Float64() < 0.13 {
			root := b.say(at, "triage", "#support", nil, fmt.Sprintf("@%s #%d from %s: %s. I cannot answer this one from the help centre, it needs a person.", name, ticket, customer, problem), "flag")
			if answer := b.fresh(supportAnswers); answer != "" {
				b.say(at.Add(time.Duration(5+b.rng.IntN(50))*time.Minute), who, "#support", root, answer, "")
			}
			continue
		}
		root := b.say(at, "triage", "#support", nil, fmt.Sprintf("#%d from %s: %s. Looks like %s, routed to %s.", ticket, customer, problem, b.pick(ticketCategories), name), "info")
		b.humanAnswer(root, 0.12)
	}
}

var supportAnswers = []string{
	"Spoke to their office manager, she is fine with a workaround until the fix ships.",
	"This is the clinic that writes in capitals. Taking it myself.",
	"Sent them the two-minute video, that usually does it.",
	"Called, no answer. Trying again after their lunch break.",
	"They had two accounts under one email. Merged them and it works.",
	"Told them we can see it and will come back today with an answer.",
	"Got it, calling them now.",
	"I know this clinic, they prefer email. Writing to them.",
	"Taking it. This is the third one this week so I will open a bug too.",
	"Answered. It was the setting under Locations, added it to the macro.",
	"On it, will update the ticket before lunch.",
	"Reproduced on staging. Passing it to engineering with the steps.",
}

func thousands(n int) string {
	if n < 1000 {
		return fmt.Sprint(n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
}

func (b *builder) metrics(day time.Time) {
	bookings := 8200 + b.rng.IntN(2400)
	reported := day.AddDate(0, 0, -1)
	if day.Weekday() == time.Monday {
		reported = day.AddDate(0, 0, -3)
	}
	if reported.Weekday() == time.Monday {
		bookings += 1400
	}
	text := fmt.Sprintf("%s: %s appointments booked, %d percent online, no-show rate %.1f percent, %d active clinics.",
		reported.Weekday(), thousands(bookings), 31+b.rng.IntN(9), 5.2+b.rng.Float64()*2.6, 214+b.rng.IntN(9))
	p := b.say(day.Add(7*time.Hour+time.Duration(50+b.rng.IntN(20))*time.Minute), "ledger", "#data", nil, text, "info")
	b.humanAnswer(p, 0.2)
	if day.Weekday() == time.Monday {
		q := b.say(day.Add(9*time.Hour+time.Duration(b.rng.IntN(20))*time.Minute), "ledger", "#sales", nil,
			fmt.Sprintf("Pipeline this week: %d open deals worth €%s a year, %d in contract, %d stalled for more than 14 days.",
				24+b.rng.IntN(10), thousands(180000+b.rng.IntN(90000)), 3+b.rng.IntN(4), 2+b.rng.IntN(4)), "")
		b.humanAnswer(q, 0.6)
	}
}

func (b *builder) research(day time.Time) {
	at := b.workMoment(day, false)
	if b.transcriptsRead >= b.transcripts {
		b.transcripts, b.transcriptsRead = 18+b.rng.IntN(14), 0
	}
	b.transcriptsRead = min(b.transcripts, b.transcriptsRead+3+b.rng.IntN(6))
	var progress string
	switch {
	case b.transcriptsRead == b.transcripts:
		progress = fmt.Sprintf("Finished all %d interview transcripts in this batch. Notes are in the research folder.", b.transcripts)
	case b.transcriptsRead*2 < b.transcripts:
		progress = fmt.Sprintf("Working through %d interview transcripts for the pilot review, %d read so far.", b.transcripts, b.transcriptsRead)
	default:
		progress = fmt.Sprintf("%d of %d transcripts read. The themes are holding, nothing new in the last few.", b.transcriptsRead, b.transcripts)
	}
	root := b.say(at, "field", "#research", nil, progress, "info")
	if b.rng.Float64() < 0.72 && len(b.findingsLeft) > 0 {
		i := b.rng.IntN(len(b.findingsLeft))
		f := b.findingsLeft[i]
		b.findingsLeft = append(b.findingsLeft[:i], b.findingsLeft[i+1:]...)
		p := b.say(at.Add(time.Duration(25+b.rng.IntN(120))*time.Minute), "field", "#research", root, f, "")
		b.humanAnswer(p, 0.45)
	}
}

func (b *builder) standup(day time.Time) {
	var parts []string
	for _, i := range b.rng.Perm(len(standupPeople))[:4+b.rng.IntN(3)] {
		who := standupPeople[i]
		parts = append(parts, fmt.Sprintf("%s: %s", who.name, b.next(who.field, workItems[who.field])))
	}
	blocked := "Nobody blocked."
	if b.rng.Float64() < 0.35 {
		who := standupPeople[b.rng.IntN(len(standupPeople))]
		blocked = fmt.Sprintf("Blocked: %s, waiting on a review.", b.next(who.field, workItems[who.field]))
	}
	p := b.say(day.Add(9*time.Hour+time.Duration(32+b.rng.IntN(10))*time.Minute), "scribe", "#engineering", nil,
		"Standup notes. "+strings.Join(parts, "; ")+". "+blocked, "info")
	b.humanAnswer(p, 0.15)
}

var standupPeople = []struct{ name, field string }{
	{"Tomás", "backend"}, {"Nadia", "backend"}, {"Arjun", "backend"}, {"Inès", "frontend"}, {"Camila", "frontend"},
	{"Kofi", "mobile"}, {"Hana", "mobile"}, {"Felix", "infra"}, {"Bram", "infra"}, {"Yuki", "qa"}, {"Leila", "qa"},
}
