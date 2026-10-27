class Atlas < Formula
  desc "Atlassian CLI for Jira, Confluence, Bitbucket, and JSM customer REST"
  homepage "https://github.com/masonhuemmer/atlas"
  url "https://github.com/masonhuemmer/atlas/archive/refs/tags/v1.5.4.tar.gz"
  sha256 "67ab7c314584806dbabfe305f4f17624b253ce3fa894b8b15abf457a4b8df2bc"
  license "MIT"
  head "https://github.com/masonhuemmer/atlas.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/atlas"
  end

  test do
    assert_match "atlas", shell_output("#{bin}/atlas --help")
    assert_match "atlas://skill", shell_output("#{bin}/atlas mcp --help")
  end
end
