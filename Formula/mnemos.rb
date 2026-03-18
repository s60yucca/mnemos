# typed: false
# frozen_string_literal: true

class Mnemos < Formula
  desc "Persistent memory engine for AI agents"
  homepage "https://github.com/mnemos-dev/mnemos"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/mnemos-dev/mnemos/releases/download/v#{version}/mnemos_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/mnemos-dev/mnemos/releases/download/v#{version}/mnemos_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/mnemos-dev/mnemos/releases/download/v#{version}/mnemos_linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/mnemos-dev/mnemos/releases/download/v#{version}/mnemos_linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_LINUX_AMD64"
    end
  end

  def install
    bin.install "mnemos"
  end

  def post_install
    (var/"mnemos").mkpath
  end

  test do
    system "#{bin}/mnemos", "version"
  end
end
