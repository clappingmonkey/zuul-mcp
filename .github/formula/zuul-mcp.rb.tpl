# typed: false
# frozen_string_literal: true

class ZuulMcp < Formula
  desc "MCP server for interacting with Zuul CI"
  homepage "https://github.com/clappingmonkey/zuul-mcp"
  version "${VERSION}"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/clappingmonkey/zuul-mcp/releases/download/v${VERSION}/zuul-mcp-darwin-arm64"
      sha256 "${DARWIN_ARM64_SHA}"
    end
    on_intel do
      url "https://github.com/clappingmonkey/zuul-mcp/releases/download/v${VERSION}/zuul-mcp-darwin-amd64"
      sha256 "${DARWIN_AMD64_SHA}"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/clappingmonkey/zuul-mcp/releases/download/v${VERSION}/zuul-mcp-linux-arm64"
      sha256 "${LINUX_ARM64_SHA}"
    end
    on_intel do
      url "https://github.com/clappingmonkey/zuul-mcp/releases/download/v${VERSION}/zuul-mcp-linux-amd64"
      sha256 "${LINUX_AMD64_SHA}"
    end
  end

  def install
    bin.install Dir["zuul-mcp-*"].first => "zuul-mcp"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/zuul-mcp --version")
  end
end
