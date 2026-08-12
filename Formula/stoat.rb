class Stoat < Formula
  desc "Security-first macOS persistence inspector and launchd manager"
  homepage "https://github.com/wuuJiawei/stoat"
  license "MIT"
  head "https://github.com/wuuJiawei/stoat.git", branch: "main"

  depends_on "go" => :build
  depends_on :macos

  def install
    ldflags = "-s -w -X main.version=#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/stoat"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/stoat version")
  end

  service do
    run [opt_bin/"stoat", "watch", "--interval", "60s", "--json"]
    keep_alive true
    process_type :background
    log_path var/"log/stoat.log"
    error_log_path var/"log/stoat.log"
  end
end
