const BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:4000";

function getAuthHeader(): Record<string, string> {
    const token = localStorage.getItem("token");
    return token ? { Authorization: `Bearer ${token}` } : {};
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const res = await fetch(`${BASE_URL}${path}`, {
        ...options,
        headers: {
            "Content-Type": "application/json",
            ...getAuthHeader(),
            ...options.headers,
        },
    });
    if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.message || "Request failed");
    }
    return res.json();
}

export default class Client {
    constructor(baseUrl: string, options?: any) {}

    auth = {
        register: (payload: any) =>
            request("/auth/register", { method: "POST", body: JSON.stringify(payload) }),
        login: (payload: any) =>
            request("/auth/login", { method: "POST", body: JSON.stringify(payload) }),
    };

    admin = {
        listQuizzes: () => request("/admin/quizzes"),
        createQuiz: (payload: any) =>
            request("/admin/quizzes", { method: "POST", body: JSON.stringify(payload) }),
        getQuiz: (id: number) => request(`/admin/quizzes/${id}`),
        updateQuiz: (id: number, payload: any) =>
            request(`/admin/quizzes/${id}`, { method: "PUT", body: JSON.stringify(payload) }),
        deleteQuiz: (id: number) =>
            request(`/admin/quizzes/${id}`, { method: "DELETE" }),
        publishQuiz: (id: number, isPublished: boolean) =>
            request(`/admin/quizzes/${id}/publish`, { method: "PATCH", body: JSON.stringify({ is_published: isPublished }) }),
    };

    quiz = {
        listQuizzes: () => request("/quizzes"),
        getQuiz: (id: number) => request(`/quizzes/${id}`),
        submit: (id: number, payload: any) =>
            request(`/quizzes/${id}/submit`, { method: "POST", body: JSON.stringify(payload) }),
        getResult: (id: number) => request(`/quizzes/${id}/result`),
    };
}