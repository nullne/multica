import { ImageResponse } from "next/og";
import { PwaIcon } from "@/components/pwa/pwa-icon";

const ALLOWED_SIZES = new Set([192, 512]);

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ size: string }> },
) {
  const { size } = await params;
  const parsedSize = Number.parseInt(size, 10);

  if (!ALLOWED_SIZES.has(parsedSize)) {
    return new Response("Unsupported icon size", { status: 404 });
  }

  return new ImageResponse(<PwaIcon size={parsedSize} />, {
    width: parsedSize,
    height: parsedSize,
  });
}
