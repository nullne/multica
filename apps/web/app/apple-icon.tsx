import { ImageResponse } from "next/og";
import { PwaIcon } from "@/components/pwa/pwa-icon";

export const size = {
  width: 180,
  height: 180,
};

export const contentType = "image/png";

export default function AppleIcon() {
  return new ImageResponse(<PwaIcon size={180} />, size);
}
