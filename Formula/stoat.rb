class Stoat < Formula
  desc "Read-only macOS persistence inspector"
  homepage "https://github.com/wuuJiawei/stoat"
  head "https://github.com/wuuJiawei/stoat.git", branch: "main"
  license "MIT"

  depends_on :macos
  depends_on "go" => :build

  def install
    ldflags = "-s -w -X main.version=#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/stoat"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/stoat version")
  end
end
