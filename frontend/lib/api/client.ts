import { readSession } from "@/lib/session";
import type {
  AdminQuizDetail,
  AuthResponse,
  LoginPayload,
  PublicQuizDetail,
  QuizListResponse,
  QuizResult,
  RegisterPayload,
  SubmitQuizPayload,
  UpsertQuizPayload,
} from "@/lib/api/types";

import Client, { Local } from "../../infrastructure/client";

const client = new Client(Local, {
    auth: () => {
        // Достаем токен из твоей функции readSession
        const session = readSession(); 
        return { token: session?.token || "" };
    }
});


const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL;

type RequestOptions = Omit<RequestInit, "body" | "headers"> & {
  body?: unknown;
  token?: string | null;
  headers?: HeadersInit;
};

type ApiErrorPayload = {
  code?: string;
  message?: string;
};

export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const session = readSession();
  const token = options.token ?? session?.token ?? null;
  const headers = new Headers(options.headers);

  if (options.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  if (!response.ok) {
    const payload = await parseErrorPayload(response);
    throw new ApiError(payload.message ?? "Request failed", response.status, payload.code);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

async function parseErrorPayload(response: Response): Promise<ApiErrorPayload> {
  try {
    const value = (await response.json()) as unknown;
    if (isApiErrorPayload(value)) {
      return value;
    }
  } catch {
    return { message: response.statusText };
  }
  return { message: response.statusText };
}

function isApiErrorPayload(value: unknown): value is ApiErrorPayload {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return (
    (record.message === undefined || typeof record.message === "string") &&
    (record.code === undefined || typeof record.code === "string")
  );
}

export const api = {
  register: (payload: RegisterPayload) =>
    client.auth.register(payload),
  login: (payload: LoginPayload) =>
    client.auth.login(payload),
  listAdminQuizzes: () => client.admin.listQuizzes(),
  createAdminQuiz: (payload: UpsertQuizPayload) =>
   client.admin.createQuiz(payload),
  getAdminQuiz: (id: number) => client.admin.getQuiz(id),
  updateAdminQuiz: (id: number, payload: UpsertQuizPayload) =>
    client.admin.updateQuiz(id, payload),
  deleteAdminQuiz: (id: number) =>
    client.admin.deleteQuiz(id),
  publishAdminQuiz: (id: number, isPublished: boolean) =>
    client.admin.publishQuiz(id, isPublished),
  listQuizzes: () => client.quiz.listQuizzes(),
  getQuiz: (id: number) => client.quiz.getQuiz(id),
  submitQuiz: (id: number, payload: SubmitQuizPayload) =>
    client.quiz.submit(id, payload),
  getQuizResult: (id: number) => client.quiz.getResult(id),
};
