package summarize

import (
	"fmt"
	"strings"

	"github.com/antalkon/ytb-sum/internal/youtube"
)

var langNames = map[string]string{
	"ru": "Russian", "en": "English", "uk": "Ukrainian", "de": "German",
	"fr": "French", "es": "Spanish", "it": "Italian", "pt": "Portuguese",
	"zh": "Chinese", "ja": "Japanese", "ko": "Korean", "tr": "Turkish",
	"pl": "Polish", "kk": "Kazakh", "be": "Belarusian",
}

// LanguageName maps a language code to an English name; unknown values pass
// through so users can write e.g. --lang "Brazilian Portuguese".
func LanguageName(code string) string {
	if n, ok := langNames[strings.ToLower(code)]; ok {
		return n
	}
	return code
}

var lengthGuides = map[string]string{
	"short":  "tldr: 1-2 sentences. key_points: 3-5. sections: 3-5, each with 1-2 points. takeaways: up to 3.",
	"medium": "tldr: 2-3 sentences. key_points: 5-8. sections: follow the video's structure (usually 4-10), each with 2-4 points. takeaways: 3-6.",
	"long":   "tldr: 3 sentences. key_points: 8-12. sections: follow the video's structure in detail, each with 3-6 points. takeaways: up to 8.",
}

func systemPrompt(lang, length string) string {
	return fmt.Sprintf(`You are an expert analyst who writes precise, information-dense summaries of YouTube videos.

Rules:
- Write every field in %[1]s, regardless of the transcript language. Keep code, commands, product and proper names in their original form.
- Use only what is said in the transcript and description. Never invent facts, numbers or claims.
- Be specific: keep concrete numbers, names, steps, arguments and conclusions. No filler like "the author talks about…" — state the substance itself.
- Skip sponsor reads, ads, greetings, requests to like/subscribe, and self-promotion.
- sections must follow the video chronologically. Each timestamp must be copied from a [M:SS] or [H:MM:SS] marker in the transcript where that part starts. If creator chapters are given, align sections with them.
- takeaways are practical conclusions or actions a viewer can apply. Use an empty array if the video has none.
- mentioned lists tools, books, people, sites or resources referred to in the video, each as "Name — short context". Use an empty array if none.
- Auto-generated captions may contain recognition errors; infer the intended words from context.
- Length: %[2]s

Respond with a single JSON object matching the provided schema and nothing else.`, LanguageName(lang), lengthGuides[length])
}

func videoContext(v *youtube.Video) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Title: %s\nChannel: %s\n", v.Title, v.Author())
	if v.Duration > 0 {
		fmt.Fprintf(&b, "Duration: %s\n", youtube.FormatTimestamp(v.Duration))
	}
	if len(v.Chapters) > 0 {
		b.WriteString("Chapters:\n")
		for _, c := range v.Chapters {
			fmt.Fprintf(&b, "  [%s] %s\n", youtube.FormatTimestamp(c.Start), c.Title)
		}
	}
	if d := strings.TrimSpace(v.Description); d != "" {
		if r := []rune(d); len(r) > 4000 {
			d = string(r[:4000]) + "…"
		}
		fmt.Fprintf(&b, "Description:\n%s\n", d)
	}
	return b.String()
}

func userPrompt(v *youtube.Video, transcript string) string {
	return videoContext(v) + "\nTranscript:\n" + transcript
}

func mapPrompt(lang string, part, total int) string {
	return fmt.Sprintf(`You are condensing part %d of %d of a long video transcript so it can be summarized later.
Write detailed notes in %s as a chronological list. Start every note with the [M:SS] marker of where it begins in the transcript.
Keep all concrete facts, numbers, names, steps and arguments. Drop ads, greetings and filler. Output plain text notes only.`, part, total, LanguageName(lang))
}
