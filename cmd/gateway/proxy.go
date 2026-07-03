package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/sapliy/sapliy-core/pkg/apierror"
)

func (h *GatewayHandler) proxyRequest(target string, w http.ResponseWriter, r *http.Request) {
	targetURL, err := url.Parse(target)
	if err != nil {
		h.logger.Error("Error parsing target URL", "target", target, "error", err)
		apierror.Internal("Internal Server Error").Write(w)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
		// Ensure headers injected by middleware persist
		if userID := r.Header.Get("X-User-ID"); userID != "" {
			req.Header.Set("X-User-ID", userID)
		}
		if env := r.Header.Get("X-Environment"); env != "" {
			req.Header.Set("X-Environment", env)
		}
		if orgID := r.Header.Get("X-Org-ID"); orgID != "" {
			req.Header.Set("X-Org-ID", orgID)
		}
		if zoneID := r.Header.Get("X-Zone-ID"); zoneID != "" {
			req.Header.Set("X-Zone-ID", zoneID)
		}
		if mode := r.Header.Get("X-Zone-Mode"); mode != "" {
			req.Header.Set("X-Zone-Mode", mode)
		}
	}

	proxy.ServeHTTP(w, r)
}
