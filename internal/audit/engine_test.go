package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/supah-seo/supah-seo/internal/crawl"
)

func TestAuditPerfectPage(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			TargetURL: "https://example.com",
			Pages: []crawl.PageResult{
				{
					URL:             "https://example.com",
					StatusCode:      200,
					Title:           "Example",
					MetaDescription: "A great example site",
					Canonical:       "https://example.com",
					Headings:        []crawl.Heading{{Level: 1, Text: "Welcome"}},
					Viewport:        "width=device-width, initial-scale=1",
					OGTitle:         "Example",
					OGDescription:   "A great example site",
					OGImage:         "https://example.com/og.png",
					Lang:            "en",
					WordCount:       500,
					SchemaTypes:     []string{"WebSite"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Score != 100 {
		t.Errorf("expected score 100 for perfect page, got %.1f", result.Score)
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected no issues, got %d: %+v", len(result.Issues), result.Issues)
	}
}

func TestAuditMissingTitle(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{URL: "https://example.com", StatusCode: 200, Headings: []crawl.Heading{{Level: 1, Text: "H"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "title-missing" {
			found = true
		}
	}
	if !found {
		t.Error("expected title-missing issue")
	}
}

func TestAuditTitleTooLong(t *testing.T) {
	svc := NewService()
	longTitle := strings.Repeat("x", 61)
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{URL: "https://example.com", StatusCode: 200, Title: longTitle, Headings: []crawl.Heading{{Level: 1, Text: "H"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "title-too-long" {
			found = true
		}
	}
	if !found {
		t.Error("expected title-too-long issue")
	}
}

func TestAuditMissingMetaDescription(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{URL: "https://example.com", StatusCode: 200, Title: "T", Headings: []crawl.Heading{{Level: 1, Text: "H"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "meta-description-missing" {
			found = true
		}
	}
	if !found {
		t.Error("expected meta-description-missing issue")
	}
}

func TestAuditMissingH1(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{URL: "https://example.com", StatusCode: 200, Title: "T", MetaDescription: "D"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "h1-missing" {
			found = true
		}
	}
	if !found {
		t.Error("expected h1-missing issue")
	}
}

func TestAuditMultipleH1(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{
					URL: "https://example.com", StatusCode: 200, Title: "T",
					Headings: []crawl.Heading{{Level: 1, Text: "A"}, {Level: 1, Text: "B"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "h1-multiple" {
			found = true
		}
	}
	if !found {
		t.Error("expected h1-multiple issue")
	}
}

func TestAuditImageMissingAlt(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{
					URL: "https://example.com", StatusCode: 200, Title: "T",
					Headings: []crawl.Heading{{Level: 1, Text: "H"}},
					Images:   []crawl.Image{{Src: "/img.png", Alt: ""}, {Src: "/ok.png", Alt: "OK"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, issue := range result.Issues {
		if issue.Rule == "img-alt-missing" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 img-alt-missing issue, got %d", count)
	}
}

func TestAuditBrokenPage(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{URL: "https://example.com/broken", StatusCode: 500},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "broken-page" && issue.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Error("expected broken-page error issue for 500 status")
	}
}

func TestAuditCanonicalMissing(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{URL: "https://example.com", StatusCode: 200, Title: "T", Headings: []crawl.Heading{{Level: 1, Text: "H"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "canonical-missing" {
			found = true
		}
	}
	if !found {
		t.Error("expected canonical-missing issue")
	}
}

// perfectPage returns a crawl.PageResult with all fields populated to avoid
// unrelated audit issues. Callers override the fields they need for the test.
func perfectPage(url, title, desc string) crawl.PageResult {
	return crawl.PageResult{
		URL:             url,
		StatusCode:      200,
		Title:           title,
		MetaDescription: desc,
		Canonical:       url,
		Headings:        []crawl.Heading{{Level: 1, Text: "Welcome"}},
		Viewport:        "width=device-width, initial-scale=1",
		OGTitle:         title,
		OGDescription:   desc,
		OGImage:         "https://example.com/og.png",
		Lang:            "en",
		WordCount:       500,
		SchemaTypes:     []string{"WebSite"},
	}
}

func TestDuplicateTitles(t *testing.T) {
	svc := NewService()
	page1 := perfectPage("https://example.com/a", "About Us", "Desc A")
	page2 := perfectPage("https://example.com/b", "About Us", "Desc B")
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			TargetURL: "https://example.com",
			Pages:     []crawl.PageResult{page1, page2},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, issue := range result.Issues {
		if issue.Rule == "duplicate-title" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 duplicate-title issues, got %d", count)
	}
}

func TestNoDuplicateTitles(t *testing.T) {
	svc := NewService()
	page1 := perfectPage("https://example.com/a", "Page A", "Desc A")
	page2 := perfectPage("https://example.com/b", "Page B", "Desc B")
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			TargetURL: "https://example.com",
			Pages:     []crawl.PageResult{page1, page2},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, issue := range result.Issues {
		if issue.Rule == "duplicate-title" {
			t.Errorf("unexpected duplicate-title issue on %s", issue.URL)
		}
	}
}

func TestDuplicateDescriptions(t *testing.T) {
	svc := NewService()
	page1 := perfectPage("https://example.com/a", "Title A", "Shared description")
	page2 := perfectPage("https://example.com/b", "Title B", "Shared description")
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			TargetURL: "https://example.com",
			Pages:     []crawl.PageResult{page1, page2},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, issue := range result.Issues {
		if issue.Rule == "duplicate-description" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 duplicate-description issues, got %d", count)
	}
}

func TestOrphanPage(t *testing.T) {
	svc := NewService()
	home := perfectPage("https://example.com", "Home", "Home desc")
	home.Links = []crawl.Link{
		{Href: "https://example.com/a", Text: "Page A", Internal: true},
	}
	pageA := perfectPage("https://example.com/a", "Page A", "Desc A")
	pageA.Links = []crawl.Link{
		{Href: "https://example.com", Text: "Home", Internal: true},
	}
	pageB := perfectPage("https://example.com/b", "Page B", "Desc B")
	// pageB is not linked from anywhere

	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			TargetURL: "https://example.com",
			Pages:     []crawl.PageResult{home, pageA, pageB},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	orphanURLs := []string{}
	for _, issue := range result.Issues {
		if issue.Rule == "orphan-page" {
			orphanURLs = append(orphanURLs, issue.URL)
		}
	}
	if len(orphanURLs) != 1 {
		t.Fatalf("expected 1 orphan-page issue, got %d: %v", len(orphanURLs), orphanURLs)
	}
	if orphanURLs[0] != "https://example.com/b" {
		t.Errorf("expected orphan URL https://example.com/b, got %s", orphanURLs[0])
	}
}

func TestNoOrphans(t *testing.T) {
	svc := NewService()
	home := perfectPage("https://example.com", "Home", "Home desc")
	home.Links = []crawl.Link{
		{Href: "https://example.com/a", Text: "Page A", Internal: true},
	}
	pageA := perfectPage("https://example.com/a", "Page A", "Desc A")
	pageA.Links = []crawl.Link{
		{Href: "https://example.com", Text: "Home", Internal: true},
	}

	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			TargetURL: "https://example.com",
			Pages:     []crawl.PageResult{home, pageA},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, issue := range result.Issues {
		if issue.Rule == "orphan-page" {
			t.Errorf("unexpected orphan-page issue on %s", issue.URL)
		}
	}
}

func TestSchemaFAQNoQuestions(t *testing.T) {
	svc := NewService()
	page := perfectPage("https://example.com/faq", "FAQ", "FAQ desc")
	page.SchemaTypes = []string{"FAQPage"}
	// No H2 headings — only the H1 from perfectPage
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			TargetURL: "https://example.com",
			Pages:     []crawl.PageResult{page},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "schema-faq-no-questions" {
			found = true
		}
	}
	if !found {
		t.Error("expected schema-faq-no-questions issue")
	}
}

func TestSchemaArticleThin(t *testing.T) {
	svc := NewService()
	page := perfectPage("https://example.com/article", "Article Title", "Article desc")
	page.SchemaTypes = []string{"Article"}
	page.WordCount = 150 // below 300 threshold
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			TargetURL: "https://example.com",
			Pages:     []crawl.PageResult{page},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "schema-article-thin" {
			found = true
		}
	}
	if !found {
		t.Error("expected schema-article-thin issue")
	}
}

func TestAuditScoreDegradesWithIssues(t *testing.T) {
	svc := NewService()
	// Page with all issues: no title, no desc, no h1, no canonical, images without alt, 500 status
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{
					URL:        "https://example.com",
					StatusCode: 500,
					Images:     []crawl.Image{{Src: "/a.png"}, {Src: "/b.png"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Score >= 100 {
		t.Errorf("expected score below 100 for page with many issues, got %.1f", result.Score)
	}
	if len(result.Issues) == 0 {
		t.Error("expected issues for broken page with missing elements")
	}
}

func TestAuditSchemaFAQNoQuestions(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{
					URL:         "https://example.com/faq",
					StatusCode:  200,
					Title:       "FAQ",
					SchemaTypes: []string{"FAQPage"},
					Headings:    []crawl.Heading{{Level: 1, Text: "FAQ"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "schema-faq-no-questions" {
			found = true
		}
	}
	if !found {
		t.Error("expected schema-faq-no-questions issue when FAQPage has no H2s")
	}
}

func TestAuditSchemaFAQWithQuestions(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{
					URL:         "https://example.com/faq",
					StatusCode:  200,
					Title:       "FAQ",
					SchemaTypes: []string{"FAQPage"},
					Headings: []crawl.Heading{
						{Level: 1, Text: "FAQ"},
						{Level: 2, Text: "What is X?"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, issue := range result.Issues {
		if issue.Rule == "schema-faq-no-questions" {
			t.Error("should not flag schema-faq-no-questions when H2s are present")
		}
	}
}

func TestAuditSchemaArticleThin(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{
					URL:         "https://example.com/post",
					StatusCode:  200,
					Title:       "Post",
					SchemaTypes: []string{"Article"},
					WordCount:   100,
					Headings:    []crawl.Heading{{Level: 1, Text: "Post"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "schema-article-thin" {
			found = true
		}
	}
	if !found {
		t.Error("expected schema-article-thin issue for Article with < 300 words")
	}
}

func TestAuditSchemaArticleOnHomepage(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{
					URL:         "https://example.com/",
					StatusCode:  200,
					Title:       "Home",
					SchemaTypes: []string{"Article"},
					WordCount:   500,
					Headings:    []crawl.Heading{{Level: 1, Text: "Home"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "schema-article-on-homepage" {
			found = true
		}
	}
	if !found {
		t.Error("expected schema-article-on-homepage issue")
	}
}

func TestAuditSchemaConflictArticleProduct(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{
					URL:         "https://example.com/item",
					StatusCode:  200,
					Title:       "Item",
					SchemaTypes: []string{"Article", "Product"},
					WordCount:   500,
					Headings:    []crawl.Heading{{Level: 1, Text: "Item"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "schema-conflict-article-product" {
			found = true
		}
	}
	if !found {
		t.Error("expected schema-conflict-article-product issue")
	}
}

func TestAuditSchemaDetectedInfoIssues(t *testing.T) {
	svc := NewService()
	result, err := svc.Run(context.Background(), Request{
		CrawlResult: crawl.Result{
			Pages: []crawl.PageResult{
				{
					URL:         "https://example.com/biz",
					StatusCode:  200,
					Title:       "Biz",
					SchemaTypes: []string{"LocalBusiness", "BreadcrumbList"},
					Headings:    []crawl.Heading{{Level: 1, Text: "Biz"}},
					WordCount:   500,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	detected := map[string]bool{}
	for _, issue := range result.Issues {
		if issue.Rule == "schema-detected-LocalBusiness" || issue.Rule == "schema-detected-BreadcrumbList" {
			detected[issue.Rule] = true
		}
	}
	if !detected["schema-detected-LocalBusiness"] {
		t.Error("expected schema-detected-LocalBusiness info issue")
	}
	if !detected["schema-detected-BreadcrumbList"] {
		t.Error("expected schema-detected-BreadcrumbList info issue")
	}
}
