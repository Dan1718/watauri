import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  distDir: process.env.NEXT_DIST_DIR || (process.env.npm_lifecycle_event === "dev:tauri" ? ".next-tauri" : process.env.NODE_ENV === "development" ? ".next-dev" : ".next"),
  output: "export",
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
