package handler

import (
	"net"
	"net/http"
)

// TrustedSubnetMiddleware пропускает запрос дальше только если IP клиента
// из заголовка X-Real-IP принадлежит доверенной подсети.
func (s *URLsService) TrustedSubnetMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.trustedSubnet == nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		ip := net.ParseIP(r.Header.Get("X-Real-IP"))
		if ip == nil || !s.trustedSubnet.Contains(ip) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
