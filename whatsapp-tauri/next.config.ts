import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  distDir: process.env.BUILD_DIST_DIR,
  output: "export",
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
