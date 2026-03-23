import { NextResponse } from "next/server";

import { asRequestRecordList, asTrendSeries, getApiBaseUrl } from "../_shared";

type DashboardDetailsResponse = {
  request_records: ReturnType<typeof asRequestRecordList>;
  token_trend: ReturnType<typeof asTrendSeries>;
  api_frequency_trend: ReturnType<typeof asTrendSeries>;
};

export async function GET() {
  try {
    getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const response: DashboardDetailsResponse = {
    request_records: asRequestRecordList(),
    token_trend: asTrendSeries(),
    api_frequency_trend: asTrendSeries(),
  };

  return NextResponse.json(response, { status: 200 });
}
