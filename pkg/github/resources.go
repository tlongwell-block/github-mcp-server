package github

import (
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterResources(s *server.MCPServer, getClient GetClientFn, denylist *RepoDenylist, t translations.TranslationHelperFunc) {
	wrap := func(tmpl mcp.ResourceTemplate, handler server.ResourceTemplateHandlerFunc) (mcp.ResourceTemplate, server.ResourceTemplateHandlerFunc) {
		if denylist != nil && !denylist.IsEmpty() {
			return tmpl, DenylistResourceGuard(denylist, handler)
		}
		return tmpl, handler
	}

	s.AddResourceTemplate(wrap(GetRepositoryResourceContent(getClient, t)))
	s.AddResourceTemplate(wrap(GetRepositoryResourceBranchContent(getClient, t)))
	s.AddResourceTemplate(wrap(GetRepositoryResourceCommitContent(getClient, t)))
	s.AddResourceTemplate(wrap(GetRepositoryResourceTagContent(getClient, t)))
	s.AddResourceTemplate(wrap(GetRepositoryResourcePrContent(getClient, t)))
}
