#!/bin/bash

ERRORS_FOUND=0

# The list of unsupported ingress-nginx annotations for traefik's compatibility layer
# https://doc.traefik.io/traefik/master/reference/routing-configuration/kubernetes/ingress-nginx/?ref=traefik.io#unsupported-nginx-annotations
FORBIDDEN_ANNOTATIONS=$(cat <<'EOF'
nginx.ingress.kubernetes.io/app-root
nginx.ingress.kubernetes.io/affinity-canary-behavior
nginx.ingress.kubernetes.io/auth-tls-secret
nginx.ingress.kubernetes.io/auth-tls-verify-depth
nginx.ingress.kubernetes.io/auth-tls-verify-client
nginx.ingress.kubernetes.io/auth-tls-error-page
nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream
nginx.ingress.kubernetes.io/auth-tls-match-cn
nginx.ingress.kubernetes.io/auth-cache-key
nginx.ingress.kubernetes.io/auth-cache-duration
nginx.ingress.kubernetes.io/auth-keepalive
nginx.ingress.kubernetes.io/auth-keepalive-share-vars
nginx.ingress.kubernetes.io/auth-keepalive-requests
nginx.ingress.kubernetes.io/auth-keepalive-timeout
nginx.ingress.kubernetes.io/auth-proxy-set-headers
nginx.ingress.kubernetes.io/auth-snippet
nginx.ingress.kubernetes.io/enable-global-auth
nginx.ingress.kubernetes.io/canary
nginx.ingress.kubernetes.io/canary-by-header
nginx.ingress.kubernetes.io/canary-by-header-value
nginx.ingress.kubernetes.io/canary-by-header-pattern
nginx.ingress.kubernetes.io/canary-by-cookie
nginx.ingress.kubernetes.io/canary-weight
nginx.ingress.kubernetes.io/canary-weight-total
nginx.ingress.kubernetes.io/client-body-buffer-size
nginx.ingress.kubernetes.io/configuration-snippet
nginx.ingress.kubernetes.io/custom-http-errors
nginx.ingress.kubernetes.io/disable-proxy-intercept-errors
nginx.ingress.kubernetes.io/default-backend
nginx.ingress.kubernetes.io/limit-rate-after
nginx.ingress.kubernetes.io/limit-rate
nginx.ingress.kubernetes.io/limit-whitelist
nginx.ingress.kubernetes.io/limit-rps
nginx.ingress.kubernetes.io/limit-rpm
nginx.ingress.kubernetes.io/limit-burst-multiplier
nginx.ingress.kubernetes.io/limit-connections
nginx.ingress.kubernetes.io/global-rate-limit
nginx.ingress.kubernetes.io/global-rate-limit-window
nginx.ingress.kubernetes.io/global-rate-limit-key
nginx.ingress.kubernetes.io/global-rate-limit-ignored-cidrs
nginx.ingress.kubernetes.io/permanent-redirect
nginx.ingress.kubernetes.io/permanent-redirect-code
nginx.ingress.kubernetes.io/temporal-redirect
nginx.ingress.kubernetes.io/preserve-trailing-slash
nginx.ingress.kubernetes.io/proxy-cookie-domain
nginx.ingress.kubernetes.io/proxy-cookie-path
nginx.ingress.kubernetes.io/proxy-connect-timeout
nginx.ingress.kubernetes.io/proxy-send-timeout
nginx.ingress.kubernetes.io/proxy-read-timeout
nginx.ingress.kubernetes.io/proxy-next-upstream
nginx.ingress.kubernetes.io/proxy-next-upstream-timeout
nginx.ingress.kubernetes.io/proxy-next-upstream-tries
nginx.ingress.kubernetes.io/proxy-request-buffering
nginx.ingress.kubernetes.io/proxy-redirect-from
nginx.ingress.kubernetes.io/proxy-redirect-to
nginx.ingress.kubernetes.io/proxy-http-version
nginx.ingress.kubernetes.io/proxy-ssl-ciphers
nginx.ingress.kubernetes.io/proxy-ssl-verify-depth
nginx.ingress.kubernetes.io/proxy-ssl-protocols
nginx.ingress.kubernetes.io/enable-rewrite-log
nginx.ingress.kubernetes.io/rewrite-target
nginx.ingress.kubernetes.io/satisfy
nginx.ingress.kubernetes.io/server-alias
nginx.ingress.kubernetes.io/server-snippet
nginx.ingress.kubernetes.io/session-cookie-conditional-samesite-none
nginx.ingress.kubernetes.io/session-cookie-expires
nginx.ingress.kubernetes.io/session-cookie-change-on-failure
nginx.ingress.kubernetes.io/ssl-ciphers
nginx.ingress.kubernetes.io/ssl-prefer-server-ciphers
nginx.ingress.kubernetes.io/connection-proxy-header
nginx.ingress.kubernetes.io/enable-access-log
nginx.ingress.kubernetes.io/enable-opentracing
nginx.ingress.kubernetes.io/opentracing-trust-incoming-span
nginx.ingress.kubernetes.io/enable-opentelemetry
nginx.ingress.kubernetes.io/opentelemetry-trust-incoming-span
nginx.ingress.kubernetes.io/enable-modsecurity
nginx.ingress.kubernetes.io/enable-owasp-core-rules
nginx.ingress.kubernetes.io/modsecurity-transaction-id
nginx.ingress.kubernetes.io/modsecurity-snippet
nginx.ingress.kubernetes.io/mirror-request-body
nginx.ingress.kubernetes.io/mirror-target
nginx.ingress.kubernetes.io/mirror-host
nginx.ingress.kubernetes.io/x-forwarded-prefix
nginx.ingress.kubernetes.io/upstream-hash-by
nginx.ingress.kubernetes.io/upstream-vhost
nginx.ingress.kubernetes.io/denylist-source-range
nginx.ingress.kubernetes.io/whitelist-source-range
nginx.ingress.kubernetes.io/proxy-buffering
nginx.ingress.kubernetes.io/proxy-buffers-number
nginx.ingress.kubernetes.io/proxy-buffer-size
nginx.ingress.kubernetes.io/proxy-max-temp-file-size
nginx.ingress.kubernetes.io/stream-snippet
EOF
)

echo "Scanning all Ingress resources across all namespaces..."

INGRESS_DATA=$(kubectl get ingress --all-namespaces -o json 2>/dev/null | \
    jq -c '.items[] | {name: .metadata.name, namespace: .metadata.namespace, annotations: .metadata.annotations}'
)

if [ -z "$INGRESS_DATA" ]; then
    echo "Could not retrieve Ingress data. Check kubectl connectivity."
    exit 1
fi

echo "---"

for ANNOTATION in $FORBIDDEN_ANNOTATIONS; do
    
    # The --arg flag passes the current shell variables into jq.
    MATCHES=$(echo "$INGRESS_DATA" | jq -r --arg a "$ANNOTATION" '
        select(.annotations | has($a)) | 
        "  - Found \($a) in Ingress \(.name) in namespace \(.namespace)"
    ')

    if [ ! -z "$MATCHES" ]; then
        echo "FORBIDDEN ANNOTATIONS FOUND:"
        echo "$MATCHES"
        ERRORS_FOUND=1
    fi
done

if [ "$ERRORS_FOUND" -eq 1 ]; then
    echo "---"
    echo "FAILED. Unsupported ingress-nginx annotations found."
    exit 1
else
    echo "PASSED. No unsupported ingress-nginx annotations found."
    exit 0
fi