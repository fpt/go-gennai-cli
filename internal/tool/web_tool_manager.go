package tool

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// WebToolManager provides web-related tools for search, navigation, and content fetching
type WebToolManager struct {
	tools map[message.ToolName]message.Tool
}

// NewWebToolManager creates a new web tool manager with all web-related tools
func NewWebToolManager() domain.ToolManager {
	m := &WebToolManager{
		tools: make(map[message.ToolName]message.Tool),
	}

	// Register all web-related tools
	m.registerWebTools()
	return m
}

func (m *WebToolManager) registerWebTools() {
	m.RegisterTool("fetch_web", "Fetch and convert a webpage to markdown format",
		[]message.ToolArgument{
			{
				Name:        "url",
				Description: "URL of the webpage to fetch and convert to markdown",
				Required:    true,
				Type:        "string",
			},
		},
		m.handleFetchWeb)
}

// Implement domain.ToolManager interface
func (m *WebToolManager) GetTool(name message.ToolName) (message.Tool, bool) {
	tool, exists := m.tools[name]
	return tool, exists
}

func (m *WebToolManager) GetTools() map[message.ToolName]message.Tool {
	return m.tools
}

func (m *WebToolManager) CallTool(ctx context.Context, name message.ToolName, args message.ToolArgumentValues) (message.ToolResult, error) {
	tool, exists := m.tools[name]
	if !exists {
		return message.NewToolResultError(fmt.Sprintf("tool '%s' not found", name)), nil
	}

	return tool.Handler()(ctx, args)
}

func (m *WebToolManager) RegisterTool(name message.ToolName, description message.ToolDescription, arguments []message.ToolArgument, handler func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)) {
	m.tools[name] = &webTool{
		name:        name,
		description: description,
		arguments:   arguments,
		handler:     handler,
	}
}

// handleFetchWeb fetches a webpage and converts it to markdown
func (m *WebToolManager) handleFetchWeb(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	urlStr, ok := args["url"].(string)
	if !ok {
		return message.NewToolResultError("url parameter is required and must be a string"), nil
	}

	// Validate and parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("invalid URL format: %v", err)), nil
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return message.NewToolResultError("invalid URL scheme: must be http or https"), nil
	}

	// Create HTTP client with timeout and proper headers
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request with proper headers
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to create request: %v", err)), nil
	}

	// Set user agent to avoid blocking
	req.Header.Set("User-Agent", "Mozilla/5.0 (Compatible Web Fetcher Bot)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	// Fetch the webpage
	resp, err := client.Do(req)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to fetch webpage: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return message.NewToolResultError(fmt.Sprintf("HTTP error %d: %s", resp.StatusCode, resp.Status)), nil
	}

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to parse HTML: %v", err)), nil
	}

	// Convert to markdown
	markdown := m.convertToMarkdown(doc, parsedURL)

	return message.NewToolResultText(markdown), nil
}

// convertToMarkdown converts HTML document to clean markdown
func (m *WebToolManager) convertToMarkdown(doc *goquery.Document, baseURL *url.URL) string {
	var result strings.Builder

	// Get page title
	title := doc.Find("title").First().Text()
	if title != "" {
		result.WriteString(fmt.Sprintf("# %s\n\n", strings.TrimSpace(title)))
	}

	// Get meta description
	metaDesc := doc.Find("meta[name='description']").AttrOr("content", "")
	if metaDesc != "" {
		result.WriteString(fmt.Sprintf("*%s*\n\n", strings.TrimSpace(metaDesc)))
	}

	// Process main content
	// Try to find main content areas first
	var contentSelectors = []string{
		"main", "article", "[role='main']", ".main-content",
		".content", ".post-content", ".article-content", "#content",
	}

	var contentFound bool
	for _, selector := range contentSelectors {
		if contentElem := doc.Find(selector).First(); contentElem.Length() > 0 {
			m.processElement(contentElem, &result, baseURL, 0)
			contentFound = true
			break
		}
	}

	// If no main content found, process body but skip navigation/footer
	if !contentFound {
		doc.Find("nav, header, footer, .navigation, .nav, .sidebar, .menu").Remove()
		m.processElement(doc.Find("body"), &result, baseURL, 0)
	}

	// Extract important links
	links := m.extractLinks(doc, baseURL)
	if len(links) > 0 {
		result.WriteString("\n## Important Links\n\n")
		for _, link := range links {
			result.WriteString(fmt.Sprintf("- [%s](%s)\n", link.Text, link.URL))
		}
	}

	return result.String()
}

// processElement recursively processes HTML elements and converts to markdown
func (m *WebToolManager) processElement(selection *goquery.Selection, result *strings.Builder, baseURL *url.URL, depth int) {
	selection.Contents().Each(func(i int, s *goquery.Selection) {
		// Handle text nodes
		if goquery.NodeName(s) == "#text" {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				result.WriteString(text)
			}
			return
		}

		// Handle HTML elements
		tagName := goquery.NodeName(s)
		switch tagName {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			level := int(tagName[1] - '0')
			result.WriteString(fmt.Sprintf("\n%s %s\n\n", strings.Repeat("#", level), strings.TrimSpace(s.Text())))

		case "p":
			text := strings.TrimSpace(s.Text())
			if text != "" {
				result.WriteString(text + "\n\n")
			}

		case "br":
			result.WriteString("\n")

		case "strong", "b":
			result.WriteString(fmt.Sprintf("**%s**", strings.TrimSpace(s.Text())))

		case "em", "i":
			result.WriteString(fmt.Sprintf("*%s*", strings.TrimSpace(s.Text())))

		case "code":
			result.WriteString(fmt.Sprintf("`%s`", strings.TrimSpace(s.Text())))

		case "pre":
			result.WriteString(fmt.Sprintf("\n```\n%s\n```\n\n", strings.TrimSpace(s.Text())))

		case "ul", "ol":
			result.WriteString("\n")
			s.Find("li").Each(func(j int, li *goquery.Selection) {
				marker := "-"
				if tagName == "ol" {
					marker = fmt.Sprintf("%d.", j+1)
				}
				result.WriteString(fmt.Sprintf("%s %s\n", marker, strings.TrimSpace(li.Text())))
			})
			result.WriteString("\n")

		case "a":
			href, exists := s.Attr("href")
			text := strings.TrimSpace(s.Text())
			if exists && text != "" {
				// Convert relative URLs to absolute
				if absoluteURL := m.resolveURL(href, baseURL); absoluteURL != "" {
					result.WriteString(fmt.Sprintf("[%s](%s)", text, absoluteURL))
				} else {
					result.WriteString(text)
				}
			} else {
				result.WriteString(text)
			}

		case "img":
			alt := s.AttrOr("alt", "Image")
			src := s.AttrOr("src", "")
			if src != "" {
				absoluteSrc := m.resolveURL(src, baseURL)
				result.WriteString(fmt.Sprintf("![%s](%s)", alt, absoluteSrc))
			}

		case "blockquote":
			lines := strings.Split(strings.TrimSpace(s.Text()), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					result.WriteString(fmt.Sprintf("> %s\n", strings.TrimSpace(line)))
				}
			}
			result.WriteString("\n")

		case "div", "span", "section", "article":
			// Process children recursively for container elements
			m.processElement(s, result, baseURL, depth+1)

		case "script", "style", "noscript":
			// Skip these elements entirely

		default:
			// For other elements, just process their text content
			text := strings.TrimSpace(s.Text())
			if text != "" {
				result.WriteString(text + " ")
			}
		}
	})
}

// resolveURL converts relative URLs to absolute URLs
func (m *WebToolManager) resolveURL(href string, baseURL *url.URL) string {
	if href == "" {
		return ""
	}

	// If already absolute, return as-is
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	// Resolve relative URL
	if resolvedURL, err := baseURL.Parse(href); err == nil {
		return resolvedURL.String()
	}

	return href
}

// Link represents a extracted link
type Link struct {
	Text string
	URL  string
}

// extractLinks extracts important links from the page
func (m *WebToolManager) extractLinks(doc *goquery.Document, baseURL *url.URL) []Link {
	var links []Link
	seen := make(map[string]bool)

	// Find important links (excluding navigation)
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		text := strings.TrimSpace(s.Text())

		// Skip empty links or navigation links
		if text == "" || len(text) > 100 {
			return
		}

		// Skip common navigation patterns
		lowerText := strings.ToLower(text)
		if strings.Contains(lowerText, "home") || strings.Contains(lowerText, "about") ||
			strings.Contains(lowerText, "contact") || strings.Contains(lowerText, "menu") {
			return
		}

		// Resolve to absolute URL
		absoluteURL := m.resolveURL(href, baseURL)
		if absoluteURL == "" || seen[absoluteURL] {
			return
		}

		seen[absoluteURL] = true
		links = append(links, Link{Text: text, URL: absoluteURL})

		// Limit to prevent overwhelming output
		if len(links) >= 10 {
			return
		}
	})

	return links
}

// webTool implements the domain.Tool interface for web tools
type webTool struct {
	name        message.ToolName
	description message.ToolDescription
	arguments   []message.ToolArgument
	handler     func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)
}

func (t *webTool) RawName() message.ToolName {
	return t.name
}

func (t *webTool) Name() message.ToolName {
	return t.name
}

func (t *webTool) Description() message.ToolDescription {
	return t.description
}

func (t *webTool) Handler() func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	return t.handler
}

func (t *webTool) Arguments() []message.ToolArgument {
	return t.arguments
}
