VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY  := ocserv-panel
DIST    := dist

.PHONY: all clean build-backend build-frontend release install-deps check

all: release

# ─── Backend ────────────────────────────────────────────────────────────────

build-backend:
	@echo "→ Building Go backend ($(VERSION))..."
	@mkdir -p $(DIST)/bin
	cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="-s -w -X main.version=$(VERSION)" \
		-o ../$(DIST)/bin/$(BINARY) ./cmd/ocserv-panel
	@echo "  ✓ $(DIST)/bin/$(BINARY)"

# ─── Frontend ───────────────────────────────────────────────────────────────

build-frontend:
	@echo "→ Building SvelteKit frontend..."
	cd frontend && pnpm install --frozen-lockfile && pnpm build
	@mkdir -p $(DIST)/static
	cp -r frontend/build/* $(DIST)/static/
	@echo "  ✓ $(DIST)/static/"

# ─── Release Tarball ────────────────────────────────────────────────────────

release: clean build-backend build-frontend
	@echo "→ Packaging release tarball..."
	@mkdir -p $(DIST)/migrations
	cp migrations/*.sql $(DIST)/migrations/
	cp deploy/ocserv-panel.service $(DIST)/
	cp deploy/install.sh $(DIST)/
	chmod +x $(DIST)/install.sh
	cp deploy/config.toml.example $(DIST)/
	echo "$(VERSION)" > $(DIST)/VERSION
	tar -czf $(BINARY)-$(VERSION)-linux-amd64.tar.gz -C $(DIST) .
	@echo "  ✓ $(BINARY)-$(VERSION)-linux-amd64.tar.gz"
	@echo ""
	@echo "Release artifact ready. Transfer to target and run:"
	@echo "  tar xzf $(BINARY)-$(VERSION)-linux-amd64.tar.gz -C /tmp/ocserv-panel-install"
	@echo "  cd /tmp/ocserv-panel-install && sudo ./install.sh"

# ─── Utilities ──────────────────────────────────────────────────────────────

clean:
	rm -rf $(DIST) $(BINARY)-*-linux-amd64.tar.gz frontend/build

check:
	@echo "→ Running backend tests..."
	cd backend && go test ./...
	@echo "→ Running frontend checks..."
	cd frontend && pnpm check

install-deps:
	cd frontend && pnpm install
