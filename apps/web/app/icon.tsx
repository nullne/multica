import { ImageResponse } from "next/og";
import { PwaIcon } from "@/components/pwa/pwa-icon";

export const size = {
  width: 512,
  height: 512,
};

export const contentType = "image/png";

export default function Icon() {
  return new ImageResponse(<PwaIcon size={512} />, size);
}
