import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api, ApiError, unwrap, type Photo, type PhotoInput, type PhotoUpload, type Plant } from "../api/client";
import { plantPhotosKey, plantsKey, todayKey } from "../api/keys";
import { preparePhoto, type PreparedPhoto } from "./prepare";

export function usePlantPhotos(plantId: string) {
  return useQuery({
    queryKey: plantPhotosKey(plantId),
    queryFn: async () =>
      unwrap(await api.GET("/plants/{id}/photos", { params: { path: { id: plantId } } })).items,
  });
}

/**
 * Covers show on plant cards, the plant hero and Today's rows. Photos sit under the plant's key,
 * so refreshing plants refreshes them too.
 */
function refreshCovers(queryClient: QueryClient) {
  return Promise.all([plantsKey, todayKey].map((queryKey) => queryClient.invalidateQueries({ queryKey })));
}

/** Uploads an already prepared photo. A plant's first photo becomes its cover. */
export async function uploadPhoto(plantId: string, { blob, takenAt }: PreparedPhoto): Promise<Photo> {
  const form = new FormData();
  form.append("file", blob, "photo.jpg");
  form.append("taken_at", takenAt);
  return unwrap(
    await api.POST("/plants/{id}/photos", {
      params: { path: { id: plantId } },
      // The schema types `file` as a string; the real body is multipart FormData.
      body: form as unknown as PhotoUpload,
      // Drop the client's JSON default so the browser sets multipart with its boundary.
      headers: { "Content-Type": null },
    }),
  );
}

/** Downscales and re-encodes the file (see prepare.ts), then uploads it. */
export function useUploadPhoto(plantId: string) {
  const queryClient = useQueryClient();
  return useMutation<Photo, Error, File>({
    mutationFn: async (file) => uploadPhoto(plantId, await preparePhoto(file)),
    onSuccess: () => refreshCovers(queryClient),
  });
}

export function useUpdatePhoto() {
  const queryClient = useQueryClient();
  return useMutation<Photo, Error, { id: string; body: PhotoInput }>({
    mutationFn: async ({ id, body }) => unwrap(await api.PUT("/photos/{id}", { params: { path: { id } }, body })),
    // The cover carries its caption and date too.
    onSuccess: () => refreshCovers(queryClient),
  });
}

export function useDeletePhoto() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: async (id) => {
      const { response, error } = await api.DELETE("/photos/{id}", { params: { path: { id } } });
      if (!response.ok) throw new ApiError(response.status, error);
    },
    // The server picks a new cover when the old one goes. Not awaited, so the viewer can move to
    // the next photo before the refreshed list drops this one (which would close it for a beat).
    onSuccess: () => void refreshCovers(queryClient),
  });
}

export function useSetCover(plantId: string) {
  const queryClient = useQueryClient();
  return useMutation<Plant, Error, string>({
    mutationFn: async (photoId) =>
      unwrap(await api.PUT("/plants/{id}/cover", { params: { path: { id: plantId } }, body: { photo_id: photoId } })),
    onSuccess: (plant) => {
      queryClient.setQueryData([...plantsKey, plant.id], plant);
      return refreshCovers(queryClient);
    },
  });
}
