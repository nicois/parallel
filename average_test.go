package parallel

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := NewRate[float64](4)
		r.Insert(10)
		_, err := r.ETA(1, 100)
		require.Error(t, err)
		time.Sleep(time.Second)
		r.Insert(20)
		_, err = r.ETA(3, 100)
		require.Error(t, err)
		eta, err := r.ETA(2, 30)
		require.NoError(t, err)
		require.Equal(t, time.Now().Add(time.Second), eta)
		eta, err = r.ETA(2, 40)
		require.NoError(t, err)
		require.Equal(t, time.Now().Add(2*time.Second), eta)
		eta, err = r.ETA(2, 100)
		require.NoError(t, err)
		require.Equal(t, time.Now().Add(8*time.Second), eta)
		time.Sleep(time.Second)
		eta, err = r.ETA(2, 100)
		require.NoError(t, err)
		require.Equal(t, time.Now().Add(7*time.Second), eta)
		r.Insert(30)
		eta, err = r.ETA(2, 100)
		require.NoError(t, err)
		// 11:00:02
		require.Equal(t, time.Now().Add(7*time.Second), eta)
		eta, err = r.ETA(3, 100)
		require.NoError(t, err)
		require.Equal(t, time.Now().Add(7*time.Second), eta)
		time.Sleep(time.Second)
		r.Insert(100)
		// So each second so far we have 20,30,100,
		// which means 40/second average.
		eta, err = r.ETA(2, 100)
		require.NoError(t, err)
		require.Equal(t, time.Now(), eta)
		eta, err = r.ETA(3, 180)
		require.NoError(t, err)
		// It should take 2 more seconds to go from 100 to 180
		require.Equal(t, time.Now().Add(2*time.Second), eta)
		eta, err = r.ETA(3, 220)
		require.NoError(t, err)
		// And an extra second to get to 220
		require.Equal(t, time.Now().Add(3*time.Second), eta)

		time.Sleep(time.Second * 3)
		eta, err = r.ETA(3, 220)
		require.NoError(t, err)
		// And an extra second to get to 220
		require.Equal(t, time.Now(), eta)

		// Now append a 110, so the last 3 samples are
		// 30,100,110 over a 4 second period, giving a
		// revised rate of 20/second
		r.Insert(110)
		eta, err = r.ETA(3, 150)
		require.NoError(t, err)
		require.Equal(t, time.Now().Add(2*time.Second), eta)
	})
}
