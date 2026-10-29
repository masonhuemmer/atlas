class Atlas < Formula
  desc "Atlassian CLI for Jira, Confluence, Bitbucket, and JSM customer REST"
  homepage "https://github.com/masonhuemmer/atlas"
  url "https://github.com/masonhuemmer/atlas/archive/refs/tags/v1.5.5.tar.gz"
  sha256 "28cebeba5c83ee2ce6eb39482332de575361eabb6641bc41a21f9a02359a5368"
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
