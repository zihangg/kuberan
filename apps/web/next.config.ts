import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  // No page may be framed by another site. This blocks clickjacking, most
  // importantly of the OAuth consent page, where a click can grant an app
  // access (including write access) to the user's data.
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Content-Security-Policy", value: "frame-ancestors 'none'" },
        ],
      },
    ];
  },
};

export default nextConfig;
