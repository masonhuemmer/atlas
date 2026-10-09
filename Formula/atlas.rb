class Atlas < Formula
  desc "Atlassian CLI for Jira, Confluence, Bitbucket, and JSM customer REST"
  homepage "https://github.com/masonhuemmer/atlas"
  url "https://github.com/masonhuemmer/atlas/archive/refs/tags/v1.3.3.tar.gz"
  sha256 "9f4278fd6fb42791d873c5cd544b970e841045ed6f8a8dd7f95af172ce88d497"
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
