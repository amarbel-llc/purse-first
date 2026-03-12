package packagebrew

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// builtinDownloadStrategy is the default custom_download_strategy.rb content
// for private GitHub repositories. It authenticates via HOMEBREW_GITHUB_API_TOKEN
// to download release assets.
const builtinDownloadStrategy = `require "download_strategy"

class GitHubPrivateRepositoryDownloadStrategy < CurlDownloadStrategy
  require "utils/github"

  def initialize(url, name, version, **meta)
    super
    parse_url_pattern
    set_github_token
  end

  def parse_url_pattern
    unless match = url.match(%r{https://github.com/([^/]+)/([^/]+)/(\S+)})
      raise CurlDownloadStrategyError, "Invalid url pattern for GitHub Repository."
    end

    _, @owner, @repo, @filepath = *match
  end

  def download_url
    "https://github.com/#{@owner}/#{@repo}/#{@filepath}"
  end

  private

  def _fetch(url:, resolved_url:, timeout:)
    curl_download download_url, "--header", "Authorization: token #{@github_token}", to: temporary_path
  end

  def set_github_token
    @github_token = GitHub::API.credentials
    unless @github_token
      raise CurlDownloadStrategyError, "No GitHub credentials found. Set HOMEBREW_GITHUB_API_TOKEN or log in with: gh auth login"
    end
  end
end

class GitHubPrivateRepositoryReleaseDownloadStrategy < GitHubPrivateRepositoryDownloadStrategy
  require "net/http"
  require "json"

  def parse_url_pattern
    url_pattern = %r{https://github.com/([^/]+)/([^/]+)/releases/download/([^/]+)/(\S+)}
    unless @url =~ url_pattern
      raise CurlDownloadStrategyError, "Invalid url pattern for GitHub Release."
    end

    _, @owner, @repo, @tag, @filename = *@url.match(url_pattern)
  end

  def download_url
    uri = URI("https://api.github.com/repos/#{@owner}/#{@repo}/releases/assets/#{asset_id}")
    req = Net::HTTP::Get.new(uri)
    req["Accept"] = "application/octet-stream"
    req["Authorization"] = "token #{@github_token}"

    res = Net::HTTP.start(uri.hostname, uri.port, use_ssl: uri.scheme == "https") do |http|
      http.request(req)
    end

    unless res["location"]
      raise CurlDownloadStrategyError, "Expected redirect for asset download, got #{res.code}"
    end

    res["location"]
  end

  private

  def _fetch(url:, resolved_url:, timeout:)
    curl_download download_url, "--header", "Accept: application/octet-stream", to: temporary_path
  end

  def asset_id
    @asset_id ||= resolve_asset_id
  end

  def resolve_asset_id
    release_metadata = fetch_release_metadata
    assets = release_metadata["assets"].select { |a| a["name"] == @filename }
    raise CurlDownloadStrategyError, "Asset file not found." if assets.empty?

    assets.first["id"]
  end

  def fetch_release_metadata
    uri = URI("https://api.github.com/repos/#{@owner}/#{@repo}/releases/tags/#{@tag}")
    req = Net::HTTP::Get.new(uri)
    req["Accept"] = "application/vnd.github+json"
    req["Authorization"] = "token #{@github_token}"

    res = Net::HTTP.start(uri.hostname, uri.port, use_ssl: true) do |http|
      http.request(req)
    end

    unless res.is_a?(Net::HTTPSuccess)
      raise CurlDownloadStrategyError, "Failed to fetch release metadata: #{res.code} #{res.message}"
    end

    JSON.parse(res.body)
  end
end
`

// writeDownloadStrategy writes the custom download strategy file to outputDir/lib/.
// If customPath is non-empty, it copies that file instead of using the built-in one.
func writeDownloadStrategy(outputDir, customPath string) error {
	libDir := filepath.Join(outputDir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return fmt.Errorf("creating lib dir: %w", err)
	}

	dest := filepath.Join(libDir, "custom_download_strategy.rb")

	if customPath != "" {
		return copyFile(customPath, dest)
	}

	return os.WriteFile(dest, []byte(builtinDownloadStrategy), 0o644)
}

func copyFile(src, dst string) (retErr error) {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("closing %s: %w", dst, cerr)
		}
	}()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying %s to %s: %w", src, dst, err)
	}

	return nil
}
