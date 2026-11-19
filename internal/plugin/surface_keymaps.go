package plugin

import (
	"errors"
	"slices"

	"github.com/erdajt/wraith/internal/keymap"
)

// setBufferKeymap layers caller-owned mappings without changing the buffer creator's lifetime.
func setBufferKeymap(buffer *surfaceBufferRecord, mutation SurfaceMutation) error {
	if mutation.Binding == nil {
		return errors.New("buffer keymap requires a binding")
	}
	binding := *mutation.Binding
	binding.Buffer = buffer.ID
	identity, err := bindingIdentity(binding)
	if err != nil {
		return err
	}
	deleted := mutation.Operation == "buffer.del_keymap"
	if !deleted {
		if _, err := keymap.New([]keymap.Binding{binding}); err != nil {
			return err
		}
	}
	if buffer.bindings == nil {
		buffer.bindings = map[string][]ownedBinding{}
	}
	layers := slices.DeleteFunc(slices.Clone(buffer.bindings[identity]), func(layer ownedBinding) bool { return layer.owner == mutation.Owner })
	layers = append(layers, ownedBinding{owner: mutation.Owner, binding: binding, deleted: deleted})
	if len(buffer.bindings) >= 512 && len(buffer.bindings[identity]) == 0 {
		return errors.New("buffer keymap limit reached")
	}
	count := len(layers)
	for id, existing := range buffer.bindings {
		if id != identity {
			count += len(existing)
		}
	}
	if count > 1024 {
		return errors.New("buffer keymap layer limit reached")
	}
	if slices.Equal(layers, buffer.bindings[identity]) {
		return nil
	}
	buffer.bindings[identity] = layers
	buffer.refreshKeymaps()
	buffer.Revision++
	return nil
}

// refreshKeymaps exposes only the highest live definition for each buffer-local sequence.
func (buffer *surfaceBufferRecord) refreshKeymaps() {
	buffer.Keymaps = nil
	var keys []string
	for key := range buffer.bindings {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		layers := buffer.bindings[key]
		if len(layers) > 0 && !layers[len(layers)-1].deleted {
			buffer.Keymaps = append(buffer.Keymaps, layers[len(layers)-1].binding)
		}
	}
}

// callbackReferences retains hidden mappings so unloading an override can restore its callback.
func (s *SurfaceStore) callbackReferences() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	retained := map[string]bool{}
	for _, buffer := range s.buffers {
		for _, layers := range buffer.bindings {
			for _, layer := range layers {
				if !layer.deleted {
					retained[layer.binding.Action] = true
				}
			}
		}
	}
	return retained
}
