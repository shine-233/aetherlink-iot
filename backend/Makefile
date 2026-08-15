# AetherLink IoT Backend - Makefile
# 覆盖率与测试命令集

.PHONY: test test-coverage test-coverage-html test-coverage-api test-coverage-router test-coverage-mqtt test-coverage-service test-coverage-dal test-coverage-utils build clean

# Go 参数
GOCMD  = go
GOBUILD = $(GOCMD) build
GOTEST = $(GOCMD) test
GOCOVER = $(GOCMD) tool cover

# 覆盖率输出
COVERAGE_DIR  = coverage
COVERAGE_OUT  = $(COVERAGE_DIR)/coverage.out
COVERAGE_HTML = $(COVERAGE_DIR)/coverage.html

# 覆盖率阈值（百分比，随测试补充逐步提升）
COVERAGE_THRESHOLD = 5

# === 通用测试 ===

test:
	$(GOTEST) ./...

# === 全量覆盖率 ===

$(COVERAGE_DIR):
	mkdir -p $(COVERAGE_DIR)

test-coverage: $(COVERAGE_DIR)
	$(GOTEST) ./... -coverprofile=$(COVERAGE_OUT) -covermode=atomic -timeout 10m
	@echo ""
	@echo "=== 覆盖率总览 ==="
	$(GOCOVER) func=$(COVERAGE_OUT) | tail -1
	@echo ""
	@echo "详细报告: make test-coverage-html"
	@echo "覆盖率文件: $(COVERAGE_OUT)"

test-coverage-html: $(COVERAGE_DIR)
	$(GOTEST) ./... -coverprofile=$(COVERAGE_OUT) -covermode=atomic -timeout 5m
	$(GOCOVER) html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@echo "HTML 报告已生成: $(COVERAGE_HTML)"

# === 分包覆盖率 ===

test-coverage-api: $(COVERAGE_DIR)
	$(GOTEST) ./internal/api/... -coverprofile=$(COVERAGE_DIR)/api.out -covermode=atomic -v -timeout 5m
	@echo ""
	@echo "=== api 层覆盖率 ==="
	$(GOCOVER) func=$(COVERAGE_DIR)/api.out | tail -1

test-coverage-router: $(COVERAGE_DIR)
	$(GOTEST) ./router/... -coverprofile=$(COVERAGE_DIR)/router.out -covermode=atomic -v -timeout 5m
	@echo ""
	@echo "=== router 层覆盖率 ==="
	$(GOCOVER) func=$(COVERAGE_DIR)/router.out | tail -1

test-coverage-mqtt: $(COVERAGE_DIR)
	$(GOTEST) ./mqtt/... ./internal/processor/... -coverprofile=$(COVERAGE_DIR)/mqtt.out -covermode=atomic -v -timeout 5m
	@echo ""
	@echo "=== mqtt/processor 覆盖率 ==="
	$(GOCOVER) func=$(COVERAGE_DIR)/mqtt.out | tail -1

test-coverage-service: $(COVERAGE_DIR)
	$(GOTEST) ./internal/service/... -coverprofile=$(COVERAGE_DIR)/service.out -covermode=atomic -v -timeout 3m
	@echo ""
	@echo "=== service 层覆盖率 ==="
	$(GOCOVER) func=$(COVERAGE_DIR)/service.out | tail -1

test-coverage-dal: $(COVERAGE_DIR)
	$(GOTEST) ./internal/dal/... -coverprofile=$(COVERAGE_DIR)/dal.out -covermode=atomic -v -timeout 3m
	@echo ""
	@echo "=== dal 层覆盖率 ==="
	$(GOCOVER) func=$(COVERAGE_DIR)/dal.out | tail -1

test-coverage-utils: $(COVERAGE_DIR)
	$(GOTEST) ./pkg/utils/... -coverprofile=$(COVERAGE_DIR)/utils.out -covermode=atomic -v -timeout 3m
	@echo ""
	@echo "=== utils 覆盖率 ==="
	$(GOCOVER) func=$(COVERAGE_DIR)/utils.out | tail -1

# === 阈值检查 ===

check-coverage: test-coverage
	@TOTAL=$$($(GOCOVER) func=$(COVERAGE_OUT) | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	if [ "$$(echo "$$TOTAL < $(COVERAGE_THRESHOLD)" | bc -l)" -eq 1 ]; then \
		echo "FAIL: 覆盖率 $$TOTAL% 低于阈值 $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	else \
		echo "PASS: 覆盖率 $$TOTAL% 达到阈值 $(COVERAGE_THRESHOLD)%"; \
	fi

# === 构建 ===

build:
	$(GOBUILD) ./...

# === 清理 ===

clean:
	rm -rf $(COVERAGE_DIR)
