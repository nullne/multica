import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Multica",
    short_name: "Multica",
    description:
      "Open-source platform that turns coding agents into real teammates. Assign tasks, track progress, compound skills.",
    start_url: "/issues",
    scope: "/",
    display: "standalone",
    orientation: "portrait",
    background_color: "#ffffff",
    theme_color: "#ffffff",
    categories: ["productivity", "business", "developer"],
    icons: [
      {
        src: "/pwa-icons/192",
        sizes: "192x192",
        type: "image/png",
      },
      {
        src: "/pwa-icons/512",
        sizes: "512x512",
        type: "image/png",
      },
      {
        src: "/pwa-icons/512?purpose=maskable",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}
