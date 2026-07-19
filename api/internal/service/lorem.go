package service

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode"
)

// loremWords is the classic lorem ipsum vocabulary.
var loremWords = []string{
	"lorem", "ipsum", "dolor", "sit", "amet", "consectetur",
	"adipiscing", "elit", "sed", "do", "eiusmod", "tempor",
	"incididunt", "ut", "labore", "et", "dolore", "magna",
	"aliqua", "enim", "ad", "minim", "veniam", "quis",
	"nostrud", "exercitation", "ullamco", "laboris", "nisi",
	"aliquip", "ex", "ea", "commodo", "consequat", "duis",
	"aute", "irure", "in", "reprehenderit", "voluptate",
	"velit", "esse", "cillum", "eu", "fugiat", "nulla",
	"pariatur", "excepteur", "sint", "occaecat", "cupidatat",
	"non", "proident", "sunt", "culpa", "qui", "officia",
	"deserunt", "mollit", "anim", "id", "est", "laborum",
	"perspiciatis", "unde", "omnis", "iste", "natus", "error",
	"voluptatem", "accusantium", "doloremque", "laudantium",
	"totam", "rem", "aperiam", "eaque", "ipsa", "quae",
	"ab", "illo", "inventore", "veritatis", "architecto",
	"beatae", "vitae", "dicta", "explicabo", "nemo",
	"ipsam", "quia", "voluptas", "aspernatur", "aut",
	"odit", "fugit", "consequuntur", "magni", "dolores",
	"ratione", "sequi", "nesciunt", "neque", "porro",
	"quisquam", "dolorem", "adipisci", "velit", "numquam",
	"eius", "modi", "tempora", "incidunt", "magnam",
	"quaerat",
}

const loremOpener = "Lorem ipsum dolor sit amet, consectetur adipiscing elit."

// RandomLorem generates `count` lorem ipsum texts.
// If sentences > 0, each text has that many sentences.
// Otherwise, each text has `paragraphs` paragraphs (3-7 sentences each).
// count must be 1-10, paragraphs 1-10, sentences 1-50.
func RandomLorem(count, paragraphs, sentences int) ([]string, error) {
	if count < 1 || count > 10 {
		return nil, fmt.Errorf("%w: count must be between 1 and 10; got %d", ErrCountOutOfRange, count)
	}
	if sentences > 0 {
		if sentences < 1 || sentences > 50 {
			return nil, fmt.Errorf("%w: sentences must be between 1 and 50; got %d", ErrInvalidParam, sentences)
		}
	} else {
		if paragraphs < 1 || paragraphs > 10 {
			return nil, fmt.Errorf("%w: paragraphs must be between 1 and 10; got %d", ErrInvalidParam, paragraphs)
		}
	}

	results := make([]string, count)
	for i := range results {
		if sentences > 0 {
			results[i] = generateLoremSentences(sentences, i == 0)
		} else {
			results[i] = generateLoremParagraphs(paragraphs, i == 0)
		}
	}
	return results, nil
}

func generateLoremSentences(n int, useOpener bool) string {
	sents := make([]string, n)
	for i := range sents {
		if i == 0 && useOpener {
			sents[i] = loremOpener
		} else {
			sents[i] = randomLoremSentence()
		}
	}
	return strings.Join(sents, " ")
}

func generateLoremParagraphs(n int, useOpener bool) string {
	paras := make([]string, n)
	for i := range paras {
		sentCount := 3 + rand.IntN(5) // 3-7 sentences
		sents := make([]string, sentCount)
		for j := range sents {
			if i == 0 && j == 0 && useOpener {
				sents[j] = loremOpener
			} else {
				sents[j] = randomLoremSentence()
			}
		}
		paras[i] = strings.Join(sents, " ")
	}
	return strings.Join(paras, "\n\n")
}

func randomLoremSentence() string {
	wordCount := 8 + rand.IntN(9) // 8-16 words
	words := make([]string, wordCount)
	for i := range words {
		words[i] = loremWords[rand.IntN(len(loremWords))]
	}
	// Capitalize first word, end with period.
	if len(words[0]) > 0 {
		runes := []rune(words[0])
		runes[0] = unicode.ToUpper(runes[0])
		words[0] = string(runes)
	}
	return strings.Join(words, " ") + "."
}
