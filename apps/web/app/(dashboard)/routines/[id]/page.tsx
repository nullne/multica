"use client";

import { use } from "react";
import { RoutineViewPage } from "../routine-view-page";

export default function RoutineDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <RoutineViewPage routineID={id} />;
}
