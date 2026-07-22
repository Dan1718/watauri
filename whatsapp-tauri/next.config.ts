import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  distDir: process.env.NEXT_DIST_DIR,
  output: "export",
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
