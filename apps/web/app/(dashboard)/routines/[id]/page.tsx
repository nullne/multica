import { redirect } from "next/navigation";

export default async function RoutineDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  redirect(`/routines?routine=${id}`);
}
